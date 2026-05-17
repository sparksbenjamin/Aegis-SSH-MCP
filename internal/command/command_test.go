package command

import "testing"

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

func TestParseNeutralizesShellOperators(t *testing.T) {
	t.Parallel()

	parsed, err := Parse(`echo hello; rm -rf /`)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}

	if got, want := parsed.Normalized, `echo 'hello;' rm -rf /`; got != want {
		t.Fatalf("unexpected normalized command: got %q want %q", got, want)
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
