package command

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/shlex"
)

var safeUnquotedToken = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,~-]+$`)

// Segment represents one command stage in a shell pipeline.
type Segment struct {
	Raw            string
	Tokens         []string
	Executable     string
	Args           []string
	Normalized     string
	NormalizedArgs string
}

// Parsed represents a shell-style command after it has been tokenized and
// normalized into a shell-safe argv form.
type Parsed struct {
	Raw            string
	Tokens         []string
	Executable     string
	Args           []string
	Normalized     string
	NormalizedArgs string
	Segments       []Segment
	HasPipes       bool
}

// Parse tokenizes a command using shell-style quoting rules, rejects control
// characters, and rebuilds a normalized command string that is safe to pass to
// ssh.Session.Run.
func Parse(raw string) (*Parsed, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("command must not be empty")
	}
	rawSegments, err := splitPipeline(raw)
	if err != nil {
		return nil, err
	}

	segments := make([]Segment, 0, len(rawSegments))
	flattenedTokens := make([]string, 0)
	for idx, rawSegment := range rawSegments {
		segment, err := parseSegment(rawSegment)
		if err != nil {
			return nil, err
		}
		segments = append(segments, segment)
		if idx > 0 {
			flattenedTokens = append(flattenedTokens, "|")
		}
		flattenedTokens = append(flattenedTokens, segment.Tokens...)
	}

	normalizedParts := make([]string, 0, len(segments))
	for _, segment := range segments {
		normalizedParts = append(normalizedParts, segment.Normalized)
	}

	first := segments[0]
	normalized := strings.Join(normalizedParts, " | ")

	return &Parsed{
		Raw:            raw,
		Tokens:         flattenedTokens,
		Executable:     first.Executable,
		Args:           append([]string(nil), first.Args...),
		Normalized:     normalized,
		NormalizedArgs: first.NormalizedArgs,
		Segments:       append([]Segment(nil), segments...),
		HasPipes:       len(segments) > 1,
	}, nil
}

func parseSegment(raw string) (Segment, error) {
	tokens, err := shlex.Split(raw)
	if err != nil {
		return Segment{}, fmt.Errorf("parse command: %w", err)
	}
	if len(tokens) == 0 {
		return Segment{}, fmt.Errorf("command must contain an executable")
	}
	if strings.TrimSpace(tokens[0]) == "" {
		return Segment{}, fmt.Errorf("command must contain a non-empty executable")
	}
	for _, token := range tokens {
		if err := validateToken(token); err != nil {
			return Segment{}, err
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

	return Segment{
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

func splitPipeline(raw string) ([]string, error) {
	inSingle := false
	inDouble := false
	escaped := false
	var current strings.Builder
	segments := make([]string, 0, 1)

	for i := 0; i < len(raw); i++ {
		ch := raw[i]

		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' && !inSingle {
			current.WriteByte(ch)
			escaped = true
			continue
		}
		if ch == '\'' && !inDouble {
			current.WriteByte(ch)
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			current.WriteByte(ch)
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble {
			current.WriteByte(ch)
			continue
		}

		switch ch {
		case '|':
			if i+1 < len(raw) && (raw[i+1] == '|' || raw[i+1] == '&') {
				return nil, fmt.Errorf("shell control operator %q is not allowed", raw[i:i+2])
			}
			segment := strings.TrimSpace(current.String())
			if segment == "" {
				return nil, fmt.Errorf("pipe operator requires a command on both sides")
			}
			segments = append(segments, segment)
			current.Reset()
		case ';', '&', '>', '<', '`':
			return nil, fmt.Errorf(
				"shell control operator %q is not allowed - use pipelines only, without redirects or command chaining",
				string(ch),
			)
		case '$':
			if i+1 < len(raw) && raw[i+1] == '(' {
				return nil, fmt.Errorf("shell command substitution $(...) is not allowed")
			}
			current.WriteByte(ch)
		default:
			current.WriteByte(ch)
		}
	}

	last := strings.TrimSpace(current.String())
	if last == "" {
		return nil, fmt.Errorf("pipe operator requires a command on both sides")
	}
	segments = append(segments, last)
	return segments, nil
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
