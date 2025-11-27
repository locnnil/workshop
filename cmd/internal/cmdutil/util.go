package cmdutil

import (
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
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

// EscapeYAMLScalar returns a YAML scalar which unmarshals to the given string.
// The intended use case is to escape user-provided data when pretty-printing
// YAML using a tabwriter. In the typical case it returns `s` unmodified. The
// result should be escaped with \xff before passing to a tabwriter. See
// https://pkg.go.dev/text/tabwriter#pkg-constants.
func EscapeYAMLScalar(s string) string {
	if strings.ContainsRune(s, '\n') {
		// YAML library sometimes produces invalid multi-line scalars.
		// See https://github.com/yaml/go-yaml/issues/76. More examples
		// can be found in the seed corpus for FuzzEscapeYAMLScalar. We
		// fall back to quotes to make the string fit on one line.
		// Originally I used json.Marshal here but it doesn't escape
		// \x7f, which seems to be required by YAML. Luckily Go only
		// uses standard escape sequences and YAML supports \x??.
		return strconv.Quote(s)
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		// This is likely unreachable, fall back to above approach.
		return strconv.Quote(s)
	}
	return strings.TrimSuffix(string(data), "\n")
}
