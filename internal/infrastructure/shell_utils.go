package infrastructure

import "strings"

// ShellEscape escapes a string for safe display in a shell command line.
// This is used for logging purposes only - exec.Command doesn't need this.
//
// The function uses single-quote escaping which is safe for all characters
// except single quotes themselves, which are handled specially.
func ShellEscape(s string) string {
	if s == "" {
		return "''"
	}

	// Check if the string contains any characters that need escaping
	needsEscape := false
	for _, c := range s {
		// Characters that have special meaning in shell
		if isShellSpecialChar(c) {
			needsEscape = true
			break
		}
	}

	if !needsEscape {
		return s
	}

	// Use single quotes, but handle embedded single quotes specially
	// Replace ' with '\'' (end quote, escaped quote, start quote)
	var result strings.Builder
	result.WriteString("'")
	for _, c := range s {
		if c == '\'' {
			result.WriteString("'\"'\"'")
		} else {
			result.WriteRune(c)
		}
	}
	result.WriteString("'")
	return result.String()
}

// ShellEscapeCommand creates a shell-safe command line string for logging.
// The binary and all arguments are properly escaped for display.
func ShellEscapeCommand(binary string, args ...string) string {
	escaped := ShellEscape(binary)
	for _, arg := range args {
		escaped += " " + ShellEscape(arg)
	}
	return escaped
}

// isShellSpecialChar returns true if the character has special meaning in shell
func isShellSpecialChar(c rune) bool {
	switch c {
	case ' ', '\t', '\'', '"', '$', '`', '\\', '!', '*', '?', '[', ']',
		'(', ')', '{', '}', '|', ';', '<', '>', '&', '~', '#', '%', '\n', '\r':
		return true
	default:
		return false
	}
}
