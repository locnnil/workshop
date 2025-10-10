package cmdutil

import (
	"os"
	"strings"
)

// ContractHome makes a path nicer and shorter by contracting $HOME to '~'.
// If the path looks like a placeholder (starts with '('), it returns "-".
//
// TODO: Make it fully correct, strings module is not path-aware.
func ContractHome(path string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if path == home || strings.HasPrefix(path, home+"/") {
			return strings.Replace(path, home, "~", 1)
		} else if strings.HasPrefix(path, "(") {
			return "-"
		}
	}
	return path
}

// EmptyDash returns "-" if the string is empty; otherwise returns the string.
func EmptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
