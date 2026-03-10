package main

import (
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPClient handles sending email replies via SMTP.
type SMTPClient struct {
	config SMTPConfig
}

// NewSMTPClient creates a new SMTP client wrapper.
func NewSMTPClient(config SMTPConfig) *SMTPClient {
	return &SMTPClient{config: config}
}

// SendReply sends an email reply to the original sender.
// It sets In-Reply-To and References headers for proper threading.
func (c *SMTPClient) SendReply(to, subject, inReplyTo, body string) error {
	// Ensure subject has "Re:" prefix.
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}

	// Build the email message with proper headers.
	var msg strings.Builder
	fmt.Fprintf(&msg, "From: %s\r\n", c.config.From)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "Subject: %s\r\n", subject)

	// Threading headers so the reply appears in the same thread.
	if inReplyTo != "" {
		fmt.Fprintf(&msg, "In-Reply-To: %s\r\n", inReplyTo)
		fmt.Fprintf(&msg, "References: %s\r\n", inReplyTo)
	}

	fmt.Fprintf(&msg, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(&msg, "\r\n")
	fmt.Fprintf(&msg, "%s\r\n", body)

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	auth := smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.Host)

	err := smtp.SendMail(addr, auth, c.config.From, []string{to}, []byte(msg.String()))
	if err != nil {
		return fmt.Errorf("sending email to %s: %w", to, err)
	}

	return nil
}
