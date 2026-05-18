package command

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/shlex"
)

var safeUnquotedToken = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,~-]+$`)

// Parsed represents a shell-style command after it has been tokenized and
// normalized into a shell-safe argv form.
type Parsed struct {
	Raw            string
	Tokens         []string
	Executable     string
	Args           []string
	Normalized     string
	NormalizedArgs string
}

// Parse tokenizes a command using shell-style quoting rules, rejects control
// characters, and rebuilds a normalized command string that is safe to pass to
// ssh.Session.Run.
func Parse(raw string) (*Parsed, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("command must not be empty")
	}
	if err := validateRawShellSyntax(raw); err != nil {
		return nil, err
	}

	tokens, err := shlex.Split(raw)
	if err != nil {
		return nil, fmt.Errorf("parse command: %w", err)
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("command must contain an executable")
	}
	if strings.TrimSpace(tokens[0]) == "" {
		return nil, fmt.Errorf("command must contain a non-empty executable")
	}
	for _, token := range tokens {
		if err := validateToken(token); err != nil {
			return nil, err
		}
	}

	normalizedTokens := make([]string, len(tokens))
	for i, token := range tokens {
		normalizedTokens[i] = shellEscape(token)
	}

	normalized := strings.Join(normalizedTokens, " ")
	normalizedArgs := ""
	if len(normalizedTokens) > 1 {
		normalizedArgs = strings.Join(normalizedTokens[1:], " ")
	}

	return &Parsed{
		Raw:            raw,
		Tokens:         append([]string(nil), tokens...),
		Executable:     tokens[0],
		Args:           append([]string(nil), tokens[1:]...),
		Normalized:     normalized,
		NormalizedArgs: normalizedArgs,
	}, nil
}

func validateToken(token string) error {
	for _, r := range token {
		if unicode.IsControl(r) {
			return fmt.Errorf("control characters are not allowed in commands")
		}
	}
	return nil
}

func validateRawShellSyntax(raw string) error {
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(raw); i++ {
		ch := raw[i]

		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble {
			continue
		}

		switch ch {
		case '|', ';', '&', '>', '<', '`':
			return fmt.Errorf(
				"shell control operator %q is not allowed - use a single command without pipes, redirects, or chaining",
				string(ch),
			)
		case '$':
			if i+1 < len(raw) && raw[i+1] == '(' {
				return fmt.Errorf("shell command substitution $(...) is not allowed")
			}
		}
	}

	return nil
}

func shellEscape(token string) string {
	if token == "" {
		return "''"
	}
	if safeUnquotedToken.MatchString(token) {
		return token
	}
	return "'" + strings.ReplaceAll(token, "'", `'"'"'`) + "'"
}
