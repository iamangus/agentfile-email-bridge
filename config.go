package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the email bridge.
type Config struct {
	Agent         string          `yaml:"agent"`
	AgentfileURL  string          `yaml:"agentfile_url"`
	MaxConcurrent int             `yaml:"max_concurrent"`
	IMAP          IMAPConfig      `yaml:"imap"`
	SMTP          SMTPConfig      `yaml:"smtp"`
	Agentfile     AgentfileConfig `yaml:"agentfile"`
}

// IMAPConfig holds IMAP connection settings.
type IMAPConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	Username     string        `yaml:"username"`
	Password     string        `yaml:"password"`
	TLS          bool          `yaml:"tls"`
	Mailbox      string        `yaml:"mailbox"`
	PollInterval time.Duration `yaml:"poll_interval"`
}

// SMTPConfig holds SMTP connection settings.
type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
}

// AgentfileConfig holds agentfile HTTP client settings.
type AgentfileConfig struct {
	Timeout time.Duration `yaml:"timeout"`
}

// envVarRegex matches ${VAR_NAME} patterns in config values.
var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// LoadConfig loads configuration with the following precedence (highest wins):
//  1. Environment variables
//  2. YAML config file (if it exists)
//  3. Built-in defaults
//
// The config file is optional. If path is empty or the file doesn't exist,
// configuration is loaded purely from env vars and defaults.
func LoadConfig(path string) (*Config, error) {
	cfg := defaultConfig()

	// Load YAML file if it exists.
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("reading config file: %w", err)
			}
			// File doesn't exist — that's fine, continue with env vars.
		} else {
			expanded := expandEnvVars(string(data))
			if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
				return nil, fmt.Errorf("parsing config YAML: %w", err)
			}
		}
	}

	// Apply environment variable overrides.
	applyEnvOverrides(cfg)

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

// defaultConfig returns a Config populated with built-in defaults.
func defaultConfig() *Config {
	return &Config{
		AgentfileURL:  "http://localhost:3000",
		MaxConcurrent: 5,
		IMAP: IMAPConfig{
			Port:         993,
			TLS:          true,
			Mailbox:      "INBOX",
			PollInterval: 30 * time.Second,
		},
		SMTP: SMTPConfig{
			Port: 587,
		},
		Agentfile: AgentfileConfig{
			Timeout: 5 * time.Minute,
		},
	}
}

// applyEnvOverrides reads environment variables and overrides any config
// field that has a corresponding env var set. Empty env vars are ignored
// (they don't blank out a value from the YAML file).
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("AGENT"); v != "" {
		cfg.Agent = v
	}
	if v := os.Getenv("AGENTFILE_URL"); v != "" {
		cfg.AgentfileURL = v
	}
	if v := os.Getenv("MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxConcurrent = n
		}
	}

	// IMAP
	if v := os.Getenv("IMAP_HOST"); v != "" {
		cfg.IMAP.Host = v
	}
	if v := os.Getenv("IMAP_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.IMAP.Port = n
		}
	}
	if v := os.Getenv("IMAP_USERNAME"); v != "" {
		cfg.IMAP.Username = v
	}
	if v := os.Getenv("IMAP_PASSWORD"); v != "" {
		cfg.IMAP.Password = v
	}
	if v := os.Getenv("IMAP_TLS"); v != "" {
		cfg.IMAP.TLS = v == "true" || v == "1"
	}
	if v := os.Getenv("IMAP_MAILBOX"); v != "" {
		cfg.IMAP.Mailbox = v
	}
	if v := os.Getenv("IMAP_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.IMAP.PollInterval = d
		}
	}

	// SMTP
	if v := os.Getenv("SMTP_HOST"); v != "" {
		cfg.SMTP.Host = v
	}
	if v := os.Getenv("SMTP_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.SMTP.Port = n
		}
	}
	if v := os.Getenv("SMTP_USERNAME"); v != "" {
		cfg.SMTP.Username = v
	}
	if v := os.Getenv("SMTP_PASSWORD"); v != "" {
		cfg.SMTP.Password = v
	}
	if v := os.Getenv("SMTP_FROM"); v != "" {
		cfg.SMTP.From = v
	}

	// Agentfile client
	if v := os.Getenv("AGENTFILE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Agentfile.Timeout = d
		}
	}
}

// expandEnvVars replaces all ${VAR_NAME} occurrences with the corresponding
// environment variable value. If the variable is not set, the placeholder
// is replaced with an empty string.
func expandEnvVars(s string) string {
	return envVarRegex.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1] // strip ${ and }
		return os.Getenv(varName)
	})
}

// validate checks that all required configuration fields are present.
func (c *Config) validate() error {
	var missing []string

	if c.Agent == "" {
		missing = append(missing, "agent (env: AGENT)")
	}
	if c.IMAP.Host == "" {
		missing = append(missing, "imap.host (env: IMAP_HOST)")
	}
	if c.IMAP.Username == "" {
		missing = append(missing, "imap.username (env: IMAP_USERNAME)")
	}
	if c.IMAP.Password == "" {
		missing = append(missing, "imap.password (env: IMAP_PASSWORD)")
	}
	if c.SMTP.Host == "" {
		missing = append(missing, "smtp.host (env: SMTP_HOST)")
	}
	if c.SMTP.Username == "" {
		missing = append(missing, "smtp.username (env: SMTP_USERNAME)")
	}
	if c.SMTP.Password == "" {
		missing = append(missing, "smtp.password (env: SMTP_PASSWORD)")
	}
	if c.SMTP.From == "" {
		missing = append(missing, "smtp.from (env: SMTP_FROM)")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}

	if c.MaxConcurrent < 1 {
		return fmt.Errorf("max_concurrent must be at least 1, got %d", c.MaxConcurrent)
	}

	return nil
}
