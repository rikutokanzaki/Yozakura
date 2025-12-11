package utils

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

func GetMotdLines(hostname string) []string {
	motdFilePath := "/config/motd.txt"

	now := time.Now().UTC().Format("Mon Jan 02 15:04:05 MST 2006")
	formattedHostname := fmt.Sprintf("%-10s", hostname+":")

	file, err := os.Open(motdFilePath)
	if err != nil {
		log.Printf("Failed to read motd file %s: %v", motdFilePath, err)
		fallbackMessage := fmt.Sprintf("Welcome. (Host: 192.168.100.3 Time: %s)", now)
		return []string{fallbackMessage}
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.ReplaceAll(line, "{now}", now)
		line = strings.ReplaceAll(line, "{hostname}", formattedHostname)
		lines = append(lines, line)
	}

	return lines
}
