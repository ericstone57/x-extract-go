package infrastructure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShellEscape_AllSpecialChars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple path",
			input:    "/tmp/simple/path",
			expected: "/tmp/simple/path",
		},
		{
			name:     "path with spaces",
			input:    "/tmp/path with spaces",
			expected: "'/tmp/path with spaces'",
		},
		{
			name:     "path with single quote",
			input:    "/tmp/path'with'quote",
			expected: "'/tmp/path'\"'\"'with'\"'\"'quote'",
		},
		{
			name:     "path with double quote",
			input:    `/tmp/path"with"quote`,
			expected: `'/tmp/path"with"quote'`,
		},
		{
			name:     "path with dollar sign",
			input:    "/tmp/path$with$dollar",
			expected: "'/tmp/path$with$dollar'",
		},
		{
			name:     "path with backtick",
			input:    "/tmp/path`with`backtick",
			expected: "'/tmp/path`with`backtick'",
		},
		{
			name:     "path with backslash",
			input:    `/tmp/path\with\backslash`,
			expected: `'/tmp/path\with\backslash'`,
		},
		{
			name:     "path with exclamation",
			input:    "/tmp/path!with!exclamation",
			expected: "'/tmp/path!with!exclamation'",
		},
		{
			name:     "path with asterisk",
			input:    "/tmp/path*with*asterisk",
			expected: "'/tmp/path*with*asterisk'",
		},
		{
			name:     "path with question mark",
			input:    "/tmp/path?with?question",
			expected: "'/tmp/path?with?question'",
		},
		{
			name:     "path with brackets",
			input:    "/tmp/path[with]brackets",
			expected: "'/tmp/path[with]brackets'",
		},
		{
			name:     "path with parentheses",
			input:    "/tmp/path(with)parens",
			expected: "'/tmp/path(with)parens'",
		},
		{
			name:     "path with braces",
			input:    "/tmp/path{with}braces",
			expected: "'/tmp/path{with}braces'",
		},
		{
			name:     "path with pipe",
			input:    "/tmp/path|with|pipe",
			expected: "'/tmp/path|with|pipe'",
		},
		{
			name:     "path with semicolon",
			input:    "/tmp/path;with;semicolon",
			expected: "'/tmp/path;with;semicolon'",
		},
		{
			name:     "path with angle brackets",
			input:    "/tmp/path<with>angles",
			expected: "'/tmp/path<with>angles'",
		},
		{
			name:     "path with ampersand",
			input:    "/tmp/path&with&ampersand",
			expected: "'/tmp/path&with&ampersand'",
		},
		{
			name:     "path with tilde",
			input:    "/tmp/path~with~tilde",
			expected: "'/tmp/path~with~tilde'",
		},
		{
			name:     "path with hash",
			input:    "/tmp/path#with#hash",
			expected: "'/tmp/path#with#hash'",
		},
		{
			name:     "path with percent",
			input:    "/tmp/path%with%percent",
			expected: "'/tmp/path%with%percent'",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "''",
		},
		{
			name:     "complex path with multiple special chars",
			input:    "/tmp/my path/with $pecial chars & more!",
			expected: "'/tmp/my path/with $pecial chars & more!'",
		},
		{
			name:     "path with single quote in middle",
			input:    "/tmp/it's a test",
			expected: `'/tmp/it'"'"'s a test'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShellEscape(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShellEscapeCommand_Complex(t *testing.T) {
	tests := []struct {
		name     string
		binary   string
		args     []string
		expected string
	}{
		{
			name:     "simple command",
			binary:   "yt-dlp",
			args:     []string{"--version"},
			expected: "yt-dlp --version",
		},
		{
			name:     "path with spaces",
			binary:   "yt-dlp",
			args:     []string{"-P", "/tmp/path with spaces"},
			expected: "yt-dlp -P '/tmp/path with spaces'",
		},
		{
			name:     "multiple args with special chars",
			binary:   "yt-dlp",
			args:     []string{"-o", "%(title)s.%(ext)s", "-P", "/tmp/my downloads", "--cookies", "/tmp/my cookies/cookies.txt"},
			expected: "yt-dlp -o '%(title)s.%(ext)s' -P '/tmp/my downloads' --cookies '/tmp/my cookies/cookies.txt'",
		},
		{
			name:     "binary with space",
			binary:   "/tmp/my apps/yt-dlp",
			args:     []string{"--version"},
			expected: "'/tmp/my apps/yt-dlp' --version",
		},
		{
			name:     "URL with query params",
			binary:   "yt-dlp",
			args:     []string{"https://x.com/user/status/123?s=20&t=abc"},
			expected: "yt-dlp 'https://x.com/user/status/123?s=20&t=abc'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShellEscapeCommand(tt.binary, tt.args...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsShellSpecialChar(t *testing.T) {
	specialChars := " \t'\"$`\\!*?[](){}|;<>&~#%\n\r"
	for _, c := range specialChars {
		assert.True(t, isShellSpecialChar(c), "Expected '%c' to be a special char", c)
	}

	normalChars := "abcABC123_-./:@=+"
	for _, c := range normalChars {
		assert.False(t, isShellSpecialChar(c), "Expected '%c' to NOT be a special char", c)
	}
}
