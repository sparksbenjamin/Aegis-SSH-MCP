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
	if parsed.HasPipes {
		t.Fatal("did not expect pipeline flag for a single command")
	}
}

func TestParseBuildsNormalizedPipeline(t *testing.T) {
	t.Parallel()

	parsed, err := Parse(`systemctl list-units --type=service --state=running | grep -iE 'jellyfin'`)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}

	if !parsed.HasPipes {
		t.Fatal("expected pipeline flag to be set")
	}
	if got, want := len(parsed.Segments), 2; got != want {
		t.Fatalf("unexpected segment count: got %d want %d", got, want)
	}
	if got, want := parsed.Normalized, `systemctl list-units --type=service --state=running | grep -iE jellyfin`; got != want {
		t.Fatalf("unexpected normalized pipeline: got %q want %q", got, want)
	}
	if got, want := parsed.Segments[1].Executable, "grep"; got != want {
		t.Fatalf("unexpected second executable: got %q want %q", got, want)
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

func TestParseRejectsCommandChainingAndRedirects(t *testing.T) {
	t.Parallel()

	cases := []string{
		`echo hello; rm -rf /`,
		`cat /etc/passwd > /tmp/out`,
		`echo hi && whoami`,
		`printf foo |& tee out`,
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(tc)
			if err == nil {
				t.Fatalf("expected parse of %q to fail", tc)
			}
			if !strings.Contains(err.Error(), "shell control operator") {
				t.Fatalf("unexpected error for %q: %v", tc, err)
			}
		})
	}
}
