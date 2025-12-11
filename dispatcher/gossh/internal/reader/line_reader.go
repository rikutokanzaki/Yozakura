package reader

import (
	"fmt"
	"gossh/internal/connector"
	"gossh/internal/utils"
	"log"
	"strings"

	"golang.org/x/crypto/ssh"
)

type LineReader struct {
	channel          ssh.Channel
	username         string
	password         string
	prompt           string
	buffer           []rune
	cursorPos        int
	escapeSeq        []byte
	prevRenderedLen  int
	history          []string
	historyIndex     int
	maxHistoryLength int
	cowrieConnector  *connector.SSHConnector
	temporaryInput   []rune
}

func NewLineReader(
	channel ssh.Channel,
	username, password, prompt string,
	history []string,
	cowrieConnector *connector.SSHConnector,
) *LineReader {
	return &LineReader{
		channel:          channel,
		username:         username,
		password:         password,
		prompt:           prompt,
		buffer:           []rune{},
		cursorPos:        0,
		escapeSeq:        []byte{},
		prevRenderedLen:  0,
		history:          history,
		historyIndex:     -1,
		maxHistoryLength: 1000,
		cowrieConnector:  cowrieConnector,
		temporaryInput:   []rune{},
	}
}

func (lr *LineReader) UpdatePrompt(newPrompt string) {
	lr.prompt = newPrompt
}

func (lr *LineReader) sendPrompt() {
	lr.channel.Write([]byte(lr.prompt))
}

func (lr *LineReader) redrawBuffer() {
	lr.channel.Write([]byte("\r\x1b[K"))
	lr.sendPrompt()

	rendered := string(lr.buffer)
	lr.channel.Write([]byte(rendered))

	back := len(lr.buffer) - lr.cursorPos
	if back > 0 {
		lr.channel.Write([]byte(fmt.Sprintf("\x1b[%dD", back)))
	}

	lr.prevRenderedLen = len(lr.buffer)
}

func (lr *LineReader) setBufferFromHistory() {
	if lr.historyIndex >= 0 && lr.historyIndex < len(lr.history) {
		lr.buffer = []rune(lr.history[lr.historyIndex])
		lr.cursorPos = len(lr.buffer)
		lr.redrawBuffer()
		lr.prevRenderedLen = len(lr.buffer)
	} else if lr.historyIndex == -1 {
		lr.buffer = append([]rune{}, lr.temporaryInput...)
		lr.cursorPos = len(lr.buffer)
		lr.redrawBuffer()
		lr.prevRenderedLen = len(lr.buffer)
	}
}

func (lr *LineReader) handleTabCompletion() {
	fullInput := string(lr.buffer)
	tokens := strings.Fields(strings.TrimSpace(fullInput))
	if len(tokens) == 0 {
		return
	}

	lastToken := tokens[len(tokens)-1]
	commandWithTab := fullInput + "\t"

	conn := lr.cowrieConnector
	if conn == nil {
		conn = connector.NewSSHConnector("cowrie", 2222)
	}

	cwd, err := conn.ReplayCwdOnly(lr.username, lr.password, lr.history)
	if err != nil {
		log.Printf("Failed to replay cwd: %v", err)
		return
	}

	command, outputChars, err := conn.ExecuteWithTab(cwd, commandWithTab, lr.username, lr.password)
	if err != nil {
		log.Printf("Failed to execute with tab: %v", err)
		return
	}

	outputCharsClean := utils.StripAnsiSequences(outputChars)
	completedCommand := utils.GetCompletionDiff(strings.TrimSpace(command), strings.TrimSpace(outputCharsClean))
	completionDiff := completedCommand[len(strings.TrimSpace(command)):]

	if completionDiff != "" {
		bufferStr := string(lr.buffer)
		tokenStart := strings.LastIndex(bufferStr, lastToken)

		if tokenStart != -1 {
			insertionIndex := tokenStart + len(lastToken)
			completionRunes := []rune(completionDiff)

			tail := append([]rune{}, lr.buffer[insertionIndex:]...)

			lr.buffer = append(lr.buffer[:insertionIndex], completionRunes...)
			lr.buffer = append(lr.buffer, tail...)

			lr.cursorPos = insertionIndex + len(completionRunes)
			lr.redrawBuffer()
		}
	}
}

func (lr *LineReader) handleEscapeSequence() {
	seq := make([]byte, 2)
	n, err := lr.channel.Read(seq)
	if err != nil || n < 2 {
		log.Printf("Failed to read escape sequence: %v", err)
		return
	}

	switch string(seq) {
	case "[A":
		if len(lr.history) > 0 {
			if lr.historyIndex == -1 {
				lr.temporaryInput = append([]rune{}, lr.buffer...)
				lr.historyIndex = len(lr.history) - 1
			} else if lr.historyIndex > 0 {
				lr.historyIndex--
			} else {
				return
			}
			lr.setBufferFromHistory()
		}

	case "[B":
		if lr.historyIndex == -1 {
			return
		}

		if lr.historyIndex < len(lr.history)-1 {
			lr.historyIndex++
			lr.setBufferFromHistory()
		} else {
			lr.historyIndex = -1
			lr.setBufferFromHistory()
			lr.temporaryInput = []rune{}
		}

	case "[C":
		if lr.cursorPos < len(lr.buffer) {
			lr.cursorPos++
			lr.channel.Write([]byte("\x1b[C"))
		}

	case "[D":
		if lr.cursorPos > 0 {
			lr.cursorPos--
			lr.channel.Write([]byte("\x1b[D"))
		}

	case "[3":
		t := make([]byte, 1)
		lr.channel.Read(t)
		if string(t) == "~" && lr.cursorPos < len(lr.buffer) {
			lr.buffer = append(lr.buffer[:lr.cursorPos], lr.buffer[lr.cursorPos+1:]...)
			if lr.cursorPos == len(lr.buffer) {
				lr.channel.Write([]byte(" \b"))
			} else {
				remainder := string(lr.buffer[lr.cursorPos:]) + " "
				lr.channel.Write([]byte(remainder))
				lr.channel.Write([]byte(fmt.Sprintf("\x1b[%dD", len(remainder))))
			}
		}
	}
}

func (lr *LineReader) Read() string {
	lr.buffer = []rune{}
	lr.cursorPos = 0
	lr.historyIndex = -1
	lr.temporaryInput = []rune{}
	lr.channel.Write([]byte("\r\x1b[2K"))
	lr.sendPrompt()

	data := make([]byte, 1)
	for {
		n, err := lr.channel.Read(data)
		if err != nil || n == 0 {
			break
		}

		b := data[0]

		if b == 0x1b {
			lr.handleEscapeSequence()
			continue
		}

		if b == '\n' || b == '\r' {
			lr.channel.Write([]byte("\r\n"))
			line := string(lr.buffer)
			if line != "" {
				lr.history = append(lr.history, line)
				if len(lr.history) > lr.maxHistoryLength {
					lr.history = lr.history[1:]
				}
			}
			return line
		}

		if b == 0x7f || b == 0x08 {
			if lr.cursorPos > 0 {
				lr.buffer = append(lr.buffer[:lr.cursorPos-1], lr.buffer[lr.cursorPos:]...)
				lr.cursorPos--
				if lr.cursorPos == len(lr.buffer) {
					lr.channel.Write([]byte("\b \b"))
				} else {
					remainder := string(lr.buffer[lr.cursorPos:]) + " "
					lr.channel.Write([]byte("\b" + remainder))
					lr.channel.Write([]byte(fmt.Sprintf("\x1b[%dD", len(remainder))))
				}
			}

			if lr.historyIndex != -1 {
				lr.temporaryInput = append([]rune{}, lr.buffer...)
			}
			continue
		}

		if b == '\t' {
			lr.handleTabCompletion()
			continue
		}

		lr.buffer = append(lr.buffer[:lr.cursorPos], append([]rune{rune(b)}, lr.buffer[lr.cursorPos:]...)...)
		lr.cursorPos++

		if lr.cursorPos == len(lr.buffer) {
			lr.channel.Write([]byte{b})
		} else {
			remainder := string(lr.buffer[lr.cursorPos-1:])
			lr.channel.Write([]byte(remainder))
			lr.channel.Write([]byte(fmt.Sprintf("\x1b[%dD", len(remainder)-1)))
		}

		if lr.historyIndex != -1 {
			lr.temporaryInput = append([]rune{}, lr.buffer...)
		}
	}

	return ""
}

func (lr *LineReader) CleanupTerminal() {
	lr.channel.Write([]byte("\x1b[0m"))
}
