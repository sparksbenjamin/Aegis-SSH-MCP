package command

import (
	"strings"
	"testing"
)

func TestParseBuildsNormalizedShellSafeCommand(t *testing.T) {
	t.Parallel()

	parsed, err := Parse(`docker ps --format "{{.Names}}"`)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}

	if parsed.Executable != "docker" {
		t.Fatalf("expected executable docker, got %q", parsed.Executable)
	}
	if got, want := parsed.Normalized, `docker ps --format '{{.Names}}'`; got != want {
		t.Fatalf("unexpected normalized command: got %q want %q", got, want)
	}
	if got, want := parsed.NormalizedArgs, `ps --format '{{.Names}}'`; got != want {
		t.Fatalf("unexpected normalized args: got %q want %q", got, want)
	}
}

func TestParseRejectsShellPipelines(t *testing.T) {
	t.Parallel()

	_, err := Parse(`systemctl list-units --type=service --state=running | grep -iE 'jellyfin'`)
	if err == nil {
		t.Fatal("expected pipe operator parse to fail")
	}
	if !strings.Contains(err.Error(), `shell control operator "|" is not allowed`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRejectsControlCharacters(t *testing.T) {
	t.Parallel()

	if _, err := Parse("printf 'bad\x01'"); err == nil {
		t.Fatal("expected control character parse to fail")
	}
}

func TestParseRejectsEmptyExecutable(t *testing.T) {
	t.Parallel()

	if _, err := Parse(`"" test`); err == nil {
		t.Fatal("expected empty executable parse to fail")
	}
}

func TestParseAllowsPipeCharacterInsideQuotedArgument(t *testing.T) {
	t.Parallel()

	parsed, err := Parse(`grep -iE 'media|server|streaming' /var/log/app.log`)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}

	if got, want := parsed.Normalized, `grep -iE 'media|server|streaming' /var/log/app.log`; got != want {
		t.Fatalf("unexpected normalized command: got %q want %q", got, want)
	}
}
