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

func shellEscape(token string) string {
	if token == "" {
		return "''"
	}
	if safeUnquotedToken.MatchString(token) {
		return token
	}
	return "'" + strings.ReplaceAll(token, "'", `'"'"'`) + "'"
}
