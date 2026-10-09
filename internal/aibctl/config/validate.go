package config

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

var contextNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// Validate checks the structural rules of a configuration file.
func Validate(cfg *Config) error {
	var errs []error
	seen := map[string]bool{}
	for i, c := range cfg.Contexts {
		field := fmt.Sprintf("contexts[%d]", i)
		if !contextNamePattern.MatchString(c.Name) {
			errs = append(errs, fmt.Errorf("%s: name %q must match %s", field, c.Name, contextNamePattern))
		}
		if seen[c.Name] {
			errs = append(errs, fmt.Errorf("%s: duplicate context name %q", field, c.Name))
		}
		seen[c.Name] = true
		errs = append(errs, validateServer(field+".server", c.Server)...)
		errs = append(errs, validateAuth(field+".auth", c.Auth)...)
		errs = append(errs, validateOutput(field+".output", c.Output)...)
		errs = append(errs, validateTimeout(field+".timeout", c.Timeout)...)
	}
	if cfg.CurrentContext != "" && !seen[cfg.CurrentContext] {
		errs = append(errs, fmt.Errorf("current_context %q does not name a configured context", cfg.CurrentContext))
	}
	errs = append(errs, validateOutput("defaults.output", cfg.Defaults.Output)...)
	errs = append(errs, validateTimeout("defaults.timeout", cfg.Defaults.Timeout)...)
	errs = append(errs, validateTimeout("defaults.token_command_timeout", cfg.Defaults.TokenCommandTimeout)...)
	return errors.Join(errs...)
}

func validateResolved(r *Resolved) error {
	var errs []error
	if r.Server == "" {
		errs = append(errs, errors.New("server is required"))
	} else {
		errs = append(errs, validateServer("server", r.Server)...)
	}
	errs = append(errs, validateAuth("auth", AuthConfig{
		Principal: r.Principal, PrincipalHeader: r.PrincipalHeader, Token: r.Token, TokenCommand: r.TokenCommand,
	})...)
	if r.Principal == "" && r.Token == "" && len(r.TokenCommand) == 0 {
		errs = append(errs, errors.New("auth: configure a principal, a token, or a token_command"))
	}
	if !slices.Contains(OutputFormats, r.Output) {
		errs = append(errs, fmt.Errorf("output %q must be one of %s", r.Output, strings.Join(OutputFormats, ", ")))
	}
	if r.Timeout <= 0 {
		errs = append(errs, errors.New("timeout must be positive"))
	}
	if r.TokenCommandTimeout <= 0 {
		errs = append(errs, errors.New("token_command_timeout must be positive"))
	}
	return errors.Join(errs...)
}

func validateServer(field, server string) []error {
	if server == "" {
		return []error{fmt.Errorf("%s is required", field)}
	}
	u, err := url.Parse(server)
	if err != nil {
		return []error{fmt.Errorf("%s: %w", field, err)}
	}
	var errs []error
	if u.Scheme != "http" && u.Scheme != "https" {
		errs = append(errs, fmt.Errorf("%s: scheme must be http or https", field))
	}
	if u.Host == "" {
		errs = append(errs, fmt.Errorf("%s: host is required", field))
	}
	if strings.Trim(u.Path, "/") != "" || u.RawQuery != "" || u.Fragment != "" {
		errs = append(errs, fmt.Errorf("%s: must not contain a path, query, or fragment", field))
	}
	return errs
}

func validateAuth(field string, a AuthConfig) []error {
	var errs []error
	if a.Token != "" && len(a.TokenCommand) > 0 {
		errs = append(errs, fmt.Errorf("%s: token and token_command are mutually exclusive", field))
	}
	if a.PrincipalHeader != "" && strings.ContainsAny(a.PrincipalHeader, " :\r\n\t") {
		errs = append(errs, fmt.Errorf("%s.principal_header: %q is not a valid header name", field, a.PrincipalHeader))
	}
	return errs
}

func validateOutput(field, output string) []error {
	if output != "" && !slices.Contains(OutputFormats, output) {
		return []error{fmt.Errorf("%s: %q must be one of %s", field, output, strings.Join(OutputFormats, ", "))}
	}
	return nil
}

func validateTimeout(field string, d time.Duration) []error {
	if d < 0 {
		return []error{fmt.Errorf("%s: must not be negative", field)}
	}
	return nil
}
