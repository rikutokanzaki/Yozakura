package utils

import (
	"regexp"
	"strings"
)

var ansiEscapeRe = regexp.MustCompile(`\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`)

func StripAnsiSequences(text string) string {
	return ansiEscapeRe.ReplaceAllString(text, "")
}

func RemovePrompt(text string) string {
	// ANSI escape sequence を探す
	re := regexp.MustCompile(`\x1b\[4.`)
	matches := re.FindAllStringIndex(text, -1)

	if len(matches) == 0 {
		// プロンプトパターンを削除
		promptRe := regexp.MustCompile(`[^@]+@[^:]+:[^$#]*[$#]\s*`)
		text = promptRe.ReplaceAllString(text, "")
		return text
	}

	lastMatch := matches[len(matches)-1]
	cutIndex := lastMatch[0]
	return text[:cutIndex]
}

func GetCompletionDiff(original, completed string) string {
	if strings.HasPrefix(completed, original) {
		return completed[len(original):]
	}
	return ""
}
