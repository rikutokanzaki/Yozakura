package connector

import (
	"bytes"
	"fmt"
	"gossh/internal/resource"
	"gossh/internal/utils"
	"io"
	"log"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHConnector struct {
	host           string
	port           int
	terminalWidth  int
	terminalHeight int
	mu             sync.RWMutex
}

func NewSSHConnector(host string, port int) *SSHConnector {
	return &SSHConnector{
		host:           host,
		port:           port,
		terminalWidth:  80,
		terminalHeight: 24,
	}
}

func (c *SSHConnector) UpdateTerminalSize(width, height int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.terminalWidth = width
	c.terminalHeight = height
}

func (c *SSHConnector) GetTerminalWidth() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.terminalWidth
}

func (c *SSHConnector) GetTerminalHeight() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.terminalHeight
}

func (c *SSHConnector) RecordLogin(username, password string) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", c.host, c.port), 10*time.Second)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resource.CloseSocket(conn)

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, fmt.Sprintf("%s:%d", c.host, c.port), config)
	if err != nil {
		if strings.Contains(err.Error(), "unable to authenticate") {
			log.Printf("Heralding auth failed (expected)")
			return nil
		}
		return fmt.Errorf("auth error: %w", err)
	}
	defer resource.CloseConnection(sshConn)

	done := make(chan struct{})
	defer close(done)

	go func() {
		for {
			select {
			case req, ok := <-reqs:
				if !ok {
					return
				}
				if req != nil && req.WantReply {
					req.Reply(false, nil)
				}
			case <-done:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case ch, ok := <-chans:
				if !ok {
					return
				}
				if ch != nil {
					ch.Reject(ssh.UnknownChannelType, "not accepting channels")
				}
			case <-done:
				return
			}
		}
	}()

	log.Printf("Heralding auth succeeded (unexpected)")
	return nil
}

func (c *SSHConnector) ReplayHistory(username, password string, history []string) (string, string, error) {
	client, session, err := c.connect(username, password)
	if err != nil {
		return "", "~", err
	}
	defer resource.CloseSSHSession(client, session)

	output := ""
	cwd := "~"

	if len(history) > 0 {
		for i, cmd := range history {
			if err := c.sendCommand(session, cmd); err != nil {
				return "", "~", err
			}
			if i == len(history)-1 {
				output, cwd, err = c.receiveUntilPrompt(session, cmd)
				if err != nil {
					return "", "~", err
				}
			}
		}
	}

	return output, cwd, nil
}

func (c *SSHConnector) ReplayCwdOnly(username, password string, history []string) (string, error) {
	client, session, err := c.connect(username, password)
	if err != nil {
		return "~", err
	}
	defer resource.CloseSSHSession(client, session)

	cwd := "~"
	for _, cmd := range history {
		if strings.HasPrefix(cmd, "cd ") {
			if err := c.sendCommand(session, cmd); err != nil {
				return "~", err
			}
			_, cwd, err = c.receiveUntilPrompt(session, cmd)
			if err != nil {
				return "~", err
			}
		}
	}

	return cwd, nil
}

func (c *SSHConnector) ExecuteCommand(command, username, password, dirCmd string) (string, string, error) {
	client, session, err := c.connect(username, password)
	if err != nil {
		return "", "~", err
	}
	defer resource.CloseSSHSession(client, session)

	if dirCmd != "" {
		if err := c.sendCommand(session, dirCmd); err != nil {
			return "", "~", err
		}
		if err := c.waitForPrompt(session); err != nil {
			return "", "~", err
		}
	}

	if err := c.sendCommand(session, command); err != nil {
		return "", "~", err
	}

	output, cwd, err := c.receiveUntilPrompt(session, command)
	return output, cwd, err
}

func (c *SSHConnector) ExecuteWithTab(cwd, command, username, password string) (string, string, error) {
	client, session, err := c.connect(username, password)
	if err != nil {
		return "", "", err
	}
	defer resource.CloseSSHSession(client, session)

	if err := c.sendCommand(session, fmt.Sprintf("cd %s", cwd)); err != nil {
		return "", "", err
	}
	if err := c.waitForPrompt(session); err != nil {
		return "", "", err
	}

	rawCommand := strings.ReplaceAll(command, "\t", "")
	if _, err := session.stdin.Write([]byte(rawCommand + "\t")); err != nil {
		return "", "", err
	}

	time.Sleep(200 * time.Millisecond)

	var output bytes.Buffer
	startTime := time.Now()
	timeout := 1 * time.Second

	buf := make([]byte, 1024)
	for {
		if time.Since(startTime) > timeout {
			break
		}

		session.stdout.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n, err := session.stdout.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			break
		}

		output.Write(buf[:n])

		decoded := output.String()
		cleaned := utils.StripAnsiSequences(decoded)

		if strings.Contains(cleaned, rawCommand) {
			index := strings.LastIndex(cleaned, rawCommand)
			if index != -1 && len(cleaned) > index+len(rawCommand) {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
	}

	return command, output.String(), nil
}

func (c *SSHConnector) connect(username, password string) (*ssh.Client, *sshSession, error) {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", c.host, c.port), config)
	if err != nil {
		return nil, nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		resource.CloseClient(client)
		return nil, nil, err
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	termWidth := c.GetTerminalWidth()
	termHeight := c.GetTerminalHeight()

	if err := session.RequestPty("xterm", termHeight, termWidth, modes); err != nil {
		resource.CloseClientAndSession(client, session)
		return nil, nil, err
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		resource.CloseClientAndSession(client, session)
		return nil, nil, err
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		resource.CloseClientAndSession(client, session)
		return nil, nil, err
	}

	if err := session.Shell(); err != nil {
		resource.CloseClientAndSession(client, session)
		return nil, nil, err
	}

	sessionWrapper := &sshSession{
		session: session,
		stdin:   stdin,
		stdout:  &timeoutReader{Reader: stdout},
	}

	if err := c.waitForPrompt(sessionWrapper); err != nil {
		resource.CloseClientAndSession(client, session)
		return nil, nil, err
	}

	return client, sessionWrapper, nil
}

type sshSession struct {
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  *timeoutReader
}

func (s *sshSession) Close() error {
	if s.session != nil {
		return s.session.Close()
	}
	return nil
}

type timeoutReader struct {
	io.Reader
}

func (t *timeoutReader) SetReadDeadline(deadline time.Time) error {
	return nil
}

func (c *SSHConnector) sendCommand(session *sshSession, cmd string) error {
	_, err := session.stdin.Write([]byte(cmd + "\n"))
	return err
}

func (c *SSHConnector) waitForPrompt(session *sshSession) error {
	buf := make([]byte, 1024)
	for {
		n, err := session.stdout.Read(buf)
		if err != nil {
			return err
		}

		data := buf[:n]
		if bytes.Contains(data, []byte("$ ")) || bytes.Contains(data, []byte("# ")) {
			break
		}
	}
	return nil
}

func (c *SSHConnector) receiveUntilPrompt(session *sshSession, sentCmd string) (string, string, error) {
	var output bytes.Buffer
	var promptLine []byte

	buf := make([]byte, 1024)
	for {
		n, err := session.stdout.Read(buf)
		if err != nil {
			return "", "~", err
		}

		data := buf[:n]
		output.Write(data)

		if bytes.Contains(data, []byte("$ ")) || bytes.Contains(data, []byte("# ")) {
			promptLine = data
			break
		}
	}

	lines := bytes.Split(output.Bytes(), []byte("\n"))
	var cleanedLines [][]byte

	cmdToCheck := strings.TrimSpace(sentCmd)

	for i, line := range lines {
		lineStr := string(line)
		trimmedLine := strings.TrimSpace(string(bytes.TrimSpace(line)))

		if trimmedLine == cmdToCheck {
			continue
		}

		if i == len(lines)-1 {
			cleanedLineStr := utils.RemovePrompt(lineStr)
			cleanedLines = append(cleanedLines, []byte(cleanedLineStr))
		} else {
			cleanedLines = append(cleanedLines, line)
		}
	}

	outputLines := string(bytes.Join(cleanedLines, []byte("\n")))

	if c.containsLsCommand(cmdToCheck) {
		outputLines = c.formatCompoundCommandOutput(outputLines, cmdToCheck)
	}

	cwd := "~"
	promptStr := string(bytes.TrimSpace(promptLine))
	re := regexp.MustCompile(`@[^:]+:(.*?)[\$#] ?`)
	if matches := re.FindStringSubmatch(promptStr); len(matches) > 1 {
		cwd = strings.TrimSpace(matches[1])
	}

	return outputLines, cwd, nil
}

func (c *SSHConnector) containsLsCommand(cmd string) bool {
	separators := []string{"&&", "||", ";", "|"}

	parts := []string{cmd}
	for _, sep := range separators {
		var newParts []string
		for _, part := range parts {
			newParts = append(newParts, strings.Split(part, sep)...)
		}
		parts = newParts
	}

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if strings.HasPrefix(trimmed, "ls") {
			afterLs := strings.TrimPrefix(trimmed, "ls")
			if afterLs == "" || afterLs[0] == ' ' || afterLs[0] == '\t' {
				return true
			}
		}
	}

	return false
}

func (c *SSHConnector) formatCompoundCommandOutput(output, cmd string) string {
	separators := []string{"&&", "||", ";", "|"}

	parts := []string{cmd}
	for _, sep := range separators {
		var newParts []string
		for _, part := range parts {
			newParts = append(newParts, strings.Split(part, sep)...)
		}
		parts = newParts
	}

	var lsCommands []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if strings.HasPrefix(trimmed, "ls") {
			afterLs := strings.TrimPrefix(trimmed, "ls")
			if afterLs == "" || afterLs[0] == ' ' || afterLs[0] == '\t' {
				lsCommands = append(lsCommands, trimmed)
			}
		}
	}

	if len(lsCommands) == 0 {
		return output
	}

	outputLines := strings.Split(output, "\n")
	var result []string
	inLsOutput := false
	var lsBuffer []string

	for _, line := range outputLines {
		trimmed := strings.TrimSpace(line)

		if !inLsOutput {
			isLsStart := false
			for _, lsCmd := range lsCommands {
				if strings.HasPrefix(trimmed, strings.Fields(lsCmd)[0]) {
					possibleItems := strings.Fields(trimmed)
					if len(possibleItems) > 0 && (possibleItems[0] == "." || possibleItems[0] == "..") {
						isLsStart = true
						break
					}
				}
			}

			if isLsStart {
				inLsOutput = true
				lsBuffer = []string{line}
			} else {
				result = append(result, line)
			}
		} else {
			if trimmed == "" || strings.Contains(trimmed, "Filesystem") ||
				strings.Contains(trimmed, "rootfs") || strings.Contains(trimmed, "udev") {
				formattedLs := c.formatLsOutput(strings.Join(lsBuffer, "\n"))
				result = append(result, strings.TrimRight(formattedLs, "\r\n"))
				result = append(result, line)
				inLsOutput = false
				lsBuffer = nil
			} else {
				lsBuffer = append(lsBuffer, line)
			}
		}
	}

	if inLsOutput && len(lsBuffer) > 0 {
		formattedLs := c.formatLsOutput(strings.Join(lsBuffer, "\n"))
		result = append(result, strings.TrimRight(formattedLs, "\r\n"))
	}

	return strings.Join(result, "\n")
}

func (c *SSHConnector) formatLsOutput(output string) string {
	lines := strings.Split(output, "\n")
	var allItems []string

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			continue
		}

		fields := strings.Fields(line)
		allItems = append(allItems, fields...)
	}

	if len(allItems) == 0 {
		return ""
	}

	maxItemLen := 0
	for _, item := range allItems {
		cleanedItem := utils.StripAnsiSequences(item)
		if len(cleanedItem) > maxItemLen {
			maxItemLen = len(cleanedItem)
		}
	}

	columnWidth := maxItemLen + 2
	if columnWidth < 11 {
		columnWidth = 11
	}

	termWidth := c.GetTerminalWidth()

	var result strings.Builder
	currentLineWidth := 0

	for i, item := range allItems {
		cleanedItem := utils.StripAnsiSequences(item)
		itemWidth := len(cleanedItem)

		if currentLineWidth > 0 && currentLineWidth+columnWidth > termWidth {
			result.WriteString("\r\n")
			currentLineWidth = 0
		}

		result.WriteString(item)
		currentLineWidth += itemWidth

		if i < len(allItems)-1 {
			padding := columnWidth - itemWidth
			result.WriteString(strings.Repeat(" ", padding))
			currentLineWidth += padding
		}
	}

	finalOutput := result.String() + "\r\n"
	return finalOutput
}
