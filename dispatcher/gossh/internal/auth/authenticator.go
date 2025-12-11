package auth

import (
	"bufio"
	"gossh/internal/resource"
	"log"
	"os"
	"strings"
)

type Authenticator struct {
	rules []authRule
}

type authRule struct {
	username string
	password string
}

func NewAuthenticator(userFile string) *Authenticator {
	a := &Authenticator{
		rules: []authRule{},
	}

	file, err := os.Open(userFile)
	if err != nil {
		log.Printf("User file '%s' not found: %v", userFile, err)
		return a
	}
	defer resource.CloseFile(file)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.Contains(line, ":") || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			a.rules = append(a.rules, authRule{
				username: parts[0],
				password: parts[1],
			})
		}
	}

	return a
}

func (a *Authenticator) Authenticate(username, password string) bool {
	for _, rule := range a.rules {
		if rule.username == username || rule.username == "*" {
			if rule.password == "*" {
				return true
			} else if strings.HasPrefix(rule.password, "!") {
				return password != rule.password[1:]
			} else {
				return password == rule.password
			}
		}
	}
	return false
}
