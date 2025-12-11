package logger

import (
	"encoding/json"
	"fmt"
	"gossh/internal/resource"
	"log"
	"os"
	"time"
)

const logFilePath = "/var/log/gossh/gossh.log"

func LogAuthEvent(addr, destAddr, username, password string, success bool) {
	srcIP, srcPort := parseAddrPair(addr)
	destIP, destPort := parseAddrPair(destAddr)

	logData := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"type":      "Gossh",
		"eventid":   "gossh.login.attempt",
		"src_ip":    srcIP,
		"src_port":  srcPort,
		"dest_ip":   destIP,
		"dest_port": destPort,
		"username":  username,
		"password":  password,
		"protocol":  "ssh",
		"success":   success,
	}

	writeLog(logData)
}

func LogCommandEvent(srcIP string, srcPort int, username, command, cwd string) {
	logData := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"type":      "Gossh",
		"eventid":   "gossh.command.input",
		"src_ip":    srcIP,
		"src_port":  srcPort,
		"username":  username,
		"command":   command,
		"cwd":       cwd,
		"protocol":  "ssh",
	}

	writeLog(logData)
}

func LogSessionClose(srcIP string, srcPort int, username string, duration time.Duration, message string) {
	logData := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"type":      "Gossh",
		"eventid":   "gossh.session.close",
		"src_ip":    srcIP,
		"src_port":  srcPort,
		"username":  username,
		"duration":  fmt.Sprintf("%.2fs", duration.Seconds()),
		"message":   message,
		"protocol":  "ssh",
	}

	writeLog(logData)
}

func writeLog(logData map[string]interface{}) {
	jsonData, err := json.Marshal(logData)
	if err != nil {
		log.Printf("Failed to marshal log data: %v", err)
		return
	}

	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to open log file: %v", err)
		return
	}
	defer resource.CloseFile(f)

	if _, err := f.Write(jsonData); err != nil {
		log.Printf("Failed to write log data: %v", err)
		return
	}

	if _, err := f.Write([]byte("\n")); err != nil {
		log.Printf("Failed to write newline: %v", err)
	}
}

func parseAddrPair(addr string) (string, int) {
	var ip string
	var port int
	fmt.Sscanf(addr, "%s:%d", &ip, &port)
	if port == 0 {
		return addr, 0
	}
	return ip, port
}
