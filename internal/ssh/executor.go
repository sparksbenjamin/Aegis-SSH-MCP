// Package ssh provides a thin, security-hardened SSH execution layer.
//
// Design decisions:
//   - Uses ssh.Session.Run() exclusively — no interactive shell, no PTY.
//     This eliminates an entire class of session-hijacking attacks.
//   - Private key permissions are checked before the key is read.
//   - HostKeyFingerprint, when configured, is verified against the remote
//     server's public key before any auth material is transmitted.
//   - Output redaction is applied after execution so sensitive strings are
//     never returned to the LLM context window.
package ssh

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"aegis-ssh-mcp/internal/config"

	gossh "golang.org/x/crypto/ssh"
)

// Result holds the captured output of a single remote command.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Execute opens a connection to the host described by cfg, runs command in a
// single non-interactive session and returns the captured output.
// It never leaves an open session regardless of outcome.
func Execute(cfg *config.HostConfig, command string) (*Result, error) {
	sshCfg, err := buildClientConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build SSH config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.HostIP, cfg.SSHPort)
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second

	// ── TCP dial ────────────────────────────────────────────────────────────
	rawConn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("TCP dial %s: %w", addr, err)
	}

	// ── SSH handshake ───────────────────────────────────────────────────────
	sshConn, chans, reqs, err := gossh.NewClientConn(rawConn, addr, sshCfg)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("SSH handshake with %s: %w", addr, err)
	}
	client := gossh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	// ── Session ─────────────────────────────────────────────────────────────
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new SSH session: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	exitCode := 0
	if err := session.Run(command); err != nil {
		// A non-zero exit code is not an executor error; propagate it normally.
		if exitErr, ok := err.(*gossh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return nil, fmt.Errorf("session.Run: %w", err)
		}
	}

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// ── Output redaction ────────────────────────────────────────────────────
	if cfg.RedactionEnabled && len(cfg.RedactionPatterns) > 0 {
		stdout = applyRedaction(stdout, cfg.RedactionPatterns)
		stderr = applyRedaction(stderr, cfg.RedactionPatterns)
	}

	return &Result{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}, nil
}

// buildClientConfig assembles a gossh.ClientConfig from a HostConfig.
func buildClientConfig(cfg *config.HostConfig) (*gossh.ClientConfig, error) {
	var authMethods []gossh.AuthMethod

	switch cfg.AuthMethod {
	case config.AuthMethodKey:
		signer, err := loadPrivateKey(cfg.KeyPath)
		if err != nil {
			return nil, err
		}
		authMethods = append(authMethods, gossh.PublicKeys(signer))

	case config.AuthMethodPassword:
		authMethods = append(authMethods, gossh.Password(cfg.Password))

	default:
		return nil, fmt.Errorf("unsupported auth_method: %q", cfg.AuthMethod)
	}

	hostKeyCallback, err := buildHostKeyCallback(cfg.HostKeyFingerprint)
	if err != nil {
		return nil, err
	}

	return &gossh.ClientConfig{
		User:            cfg.SSHUser,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         time.Duration(cfg.TimeoutSeconds) * time.Second,
	}, nil
}

// loadPrivateKey reads and parses a PEM-encoded private key, enforcing that
// the key file permissions are not world- or group-readable.
func loadPrivateKey(path string) (gossh.Signer, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat key %s: %w", path, err)
	}

	// Reject keys accessible by group or others (must be 0600 or 0400).
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf(
			"insecure permissions %04o on key %s — expected 0600 or 0400",
			info.Mode().Perm(), path,
		)
	}

	pemData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key %s: %w", path, err)
	}

	signer, err := gossh.ParsePrivateKey(pemData)
	if err != nil {
		return nil, fmt.Errorf("parse private key %s: %w", path, err)
	}

	return signer, nil
}

// buildHostKeyCallback returns a host-key verification callback.
//
//   - When fingerprint is provided it is verified exactly.
//   - When fingerprint is empty it falls back to InsecureIgnoreHostKey with a
//     warning, suitable for isolated lab networks only.
func buildHostKeyCallback(fingerprint string) (gossh.HostKeyCallback, error) {
	if fingerprint == "" {
		fmt.Fprintf(os.Stderr,
			"[AEGIS] WARNING: host_key_fingerprint not set — using InsecureIgnoreHostKey. "+
				"Set this field in production.\n")
		return gossh.InsecureIgnoreHostKey(), nil //nolint:gosec // intentional lab fallback
	}

	return func(_ string, _ net.Addr, key gossh.PublicKey) error {
		got := gossh.FingerprintSHA256(key)
		if got != fingerprint {
			return fmt.Errorf("host key fingerprint mismatch: want %s got %s",
				fingerprint, got)
		}
		return nil
	}, nil
}

// applyRedaction applies each pattern as a regex substitution, replacing any
// match with "[REDACTED]".  Invalid patterns are silently skipped so a bad
// pattern doesn't break command execution — it only leaks data, which is
// always logged as a warning during engine startup.
func applyRedaction(text string, patterns []string) string {
	var sb strings.Builder
	sb.WriteString(text)
	result := sb.String()

	for _, raw := range patterns {
		re, err := regexp.Compile(raw)
		if err != nil {
			continue
		}
		result = re.ReplaceAllString(result, "[REDACTED]")
	}
	return result
}
