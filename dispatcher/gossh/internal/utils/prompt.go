package utils

import "fmt"

func GetPrompt(username, hostname, cwd string) string {
	return fmt.Sprintf("%s@%s:%s# ", username, hostname, cwd)
}
