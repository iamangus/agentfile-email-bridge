package main

import (
	"fmt"
	"os"
	"regexp"
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

// LoadConfig reads and parses a YAML config file, expanding environment
// variable references of the form ${VAR_NAME}.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Expand ${ENV_VAR} references before parsing YAML.
	expanded := expandEnvVars(string(data))

	cfg := &Config{
		// Defaults
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

	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("parsing config YAML: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
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
		missing = append(missing, "agent")
	}
	if c.IMAP.Host == "" {
		missing = append(missing, "imap.host")
	}
	if c.IMAP.Username == "" {
		missing = append(missing, "imap.username")
	}
	if c.IMAP.Password == "" {
		missing = append(missing, "imap.password")
	}
	if c.SMTP.Host == "" {
		missing = append(missing, "smtp.host")
	}
	if c.SMTP.Username == "" {
		missing = append(missing, "smtp.username")
	}
	if c.SMTP.Password == "" {
		missing = append(missing, "smtp.password")
	}
	if c.SMTP.From == "" {
		missing = append(missing, "smtp.from")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}

	if c.MaxConcurrent < 1 {
		return fmt.Errorf("max_concurrent must be at least 1, got %d", c.MaxConcurrent)
	}

	return nil
}
