package rules

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"aegis-ssh-mcp/internal/command"
)

// Profile is the in-memory representation of a rules/*.json security profile.
type Profile struct {
	ProfileName              string   `json:"profile_name"`
	WhitelistRegex           []string `json:"whitelist_regex"`
	BlacklistRegex           []string `json:"blacklist_regex"`
	ExecutableWhitelistRegex []string `json:"executable_whitelist_regex"`
	ExecutableBlacklistRegex []string `json:"executable_blacklist_regex"`
	ArgumentsWhitelistRegex  []string `json:"arguments_whitelist_regex"`
	ArgumentsBlacklistRegex  []string `json:"arguments_blacklist_regex"`

	compiledWhitelist           []*regexp.Regexp
	compiledBlacklist           []*regexp.Regexp
	compiledExecutableWhitelist []*regexp.Regexp
	compiledExecutableBlacklist []*regexp.Regexp
	compiledArgumentsWhitelist  []*regexp.Regexp
	compiledArgumentsBlacklist  []*regexp.Regexp
}

// ValidationResult is the output of Engine.Validate.
type ValidationResult struct {
	Passed bool
	Reason string
}

// Engine loads and caches all rule profiles and validates commands against them.
type Engine struct {
	mu       sync.RWMutex
	profiles map[string]*Profile
	rulesDir string
}

// NewEngine scans rulesDir for JSON profiles, compiles their regexes, and
// returns a ready-to-use Engine.
func NewEngine(rulesDir string) (*Engine, error) {
	e := &Engine{
		profiles: make(map[string]*Profile),
		rulesDir: rulesDir,
	}
	if err := e.LoadAll(); err != nil {
		return nil, err
	}
	return e, nil
}

// LoadAll rebuilds the rules cache from disk.
func (e *Engine) LoadAll() error {
	if _, err := os.Stat(e.rulesDir); os.IsNotExist(err) {
		return fmt.Errorf("rules directory %q does not exist", e.rulesDir)
	}

	entries, err := os.ReadDir(e.rulesDir)
	if err != nil {
		return fmt.Errorf("reading rules directory %s: %w", e.rulesDir, err)
	}

	loaded := make(map[string]*Profile)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(e.rulesDir, entry.Name())
		profile, err := loadProfile(path)
		if err != nil {
			return err
		}
		if _, exists := loaded[profile.ProfileName]; exists {
			return fmt.Errorf("duplicate rule profile %q", profile.ProfileName)
		}

		loaded[profile.ProfileName] = profile
		fmt.Fprintf(
			os.Stderr,
			"[AEGIS] Loaded rule profile %q (%d whitelist, %d blacklist patterns)\n",
			profile.ProfileName,
			len(profile.compiledWhitelist),
			len(profile.compiledBlacklist),
		)
	}

	e.mu.Lock()
	e.profiles = loaded
	e.mu.Unlock()

	return nil
}

func loadProfile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if strings.TrimSpace(p.ProfileName) == "" {
		return nil, fmt.Errorf("rules file %s missing required field: profile_name", path)
	}

	for _, raw := range p.WhitelistRegex {
		re, err := regexp.Compile(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid whitelist regex %q in %s: %w", raw, path, err)
		}
		p.compiledWhitelist = append(p.compiledWhitelist, re)
	}
	for _, raw := range p.BlacklistRegex {
		re, err := regexp.Compile(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid blacklist regex %q in %s: %w", raw, path, err)
		}
		p.compiledBlacklist = append(p.compiledBlacklist, re)
	}
	for _, raw := range p.ExecutableWhitelistRegex {
		re, err := regexp.Compile(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid executable whitelist regex %q in %s: %w", raw, path, err)
		}
		p.compiledExecutableWhitelist = append(p.compiledExecutableWhitelist, re)
	}
	for _, raw := range p.ExecutableBlacklistRegex {
		re, err := regexp.Compile(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid executable blacklist regex %q in %s: %w", raw, path, err)
		}
		p.compiledExecutableBlacklist = append(p.compiledExecutableBlacklist, re)
	}
	for _, raw := range p.ArgumentsWhitelistRegex {
		re, err := regexp.Compile(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid arguments whitelist regex %q in %s: %w", raw, path, err)
		}
		p.compiledArgumentsWhitelist = append(p.compiledArgumentsWhitelist, re)
	}
	for _, raw := range p.ArgumentsBlacklistRegex {
		re, err := regexp.Compile(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid arguments blacklist regex %q in %s: %w", raw, path, err)
		}
		p.compiledArgumentsBlacklist = append(p.compiledArgumentsBlacklist, re)
	}

	return &p, nil
}

// Validate runs the blacklist first, then the whitelist for the named profile.
func (e *Engine) Validate(profileName string, cmd *command.Parsed) ValidationResult {
	e.mu.RLock()
	profile, ok := e.profiles[profileName]
	e.mu.RUnlock()

	if !ok {
		return ValidationResult{
			Passed: false,
			Reason: fmt.Sprintf("unknown rule profile %q - check the rules directory", profileName),
		}
	}

	for _, re := range profile.compiledExecutableBlacklist {
		if re.MatchString(cmd.Executable) {
			return ValidationResult{
				Passed: false,
				Reason: fmt.Sprintf("executable %q matches blacklist pattern /%s/", cmd.Executable, re.String()),
			}
		}
	}

	for _, re := range profile.compiledArgumentsBlacklist {
		if re.MatchString(cmd.NormalizedArgs) {
			return ValidationResult{
				Passed: false,
				Reason: fmt.Sprintf("arguments match blacklist pattern /%s/", re.String()),
			}
		}
	}

	for _, re := range profile.compiledBlacklist {
		if re.MatchString(cmd.Normalized) {
			return ValidationResult{
				Passed: false,
				Reason: fmt.Sprintf("matches blacklist pattern /%s/", re.String()),
			}
		}
	}

	if len(profile.compiledExecutableWhitelist) > 0 {
		matched := false
		for _, re := range profile.compiledExecutableWhitelist {
			if re.MatchString(cmd.Executable) {
				matched = true
				break
			}
		}
		if !matched {
			return ValidationResult{
				Passed: false,
				Reason: "executable does not match any whitelist pattern in profile " + profileName,
			}
		}
	}

	if len(profile.compiledArgumentsWhitelist) > 0 {
		matched := false
		for _, re := range profile.compiledArgumentsWhitelist {
			if re.MatchString(cmd.NormalizedArgs) {
				matched = true
				break
			}
		}
		if !matched {
			return ValidationResult{
				Passed: false,
				Reason: "arguments do not match any whitelist pattern in profile " + profileName,
			}
		}
	}

	if len(profile.compiledWhitelist) > 0 {
		matched := false
		for _, re := range profile.compiledWhitelist {
			if re.MatchString(cmd.Normalized) {
				matched = true
				break
			}
		}
		if !matched {
			return ValidationResult{
				Passed: false,
				Reason: "command does not match any whitelist pattern in profile " + profileName,
			}
		}
	}

	return ValidationResult{Passed: true, Reason: "OK"}
}

// ProfileNames returns a sorted list of all loaded profile names.
func (e *Engine) ProfileNames() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	names := make([]string, 0, len(e.profiles))
	for k := range e.profiles {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
