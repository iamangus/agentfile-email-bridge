package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
)

// Email represents a fetched email message.
type Email struct {
	UID       imap.UID
	From      string
	Subject   string
	Body      string
	MessageID string
}

// IMAPClient handles IMAP connections and email fetching.
type IMAPClient struct {
	config IMAPConfig
	client *imapclient.Client
}

// NewIMAPClient creates a new IMAP client wrapper.
func NewIMAPClient(config IMAPConfig) *IMAPClient {
	return &IMAPClient{config: config}
}

// Connect establishes a TLS connection to the IMAP server and authenticates.
func (c *IMAPClient) Connect() error {
	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	var client *imapclient.Client
	var err error

	if c.config.TLS {
		client, err = imapclient.DialTLS(addr, nil)
	} else {
		client, err = imapclient.DialInsecure(addr, nil)
	}
	if err != nil {
		return fmt.Errorf("connecting to IMAP server %s: %w", addr, err)
	}

	if err := client.Login(c.config.Username, c.config.Password).Wait(); err != nil {
		client.Close()
		return fmt.Errorf("IMAP login: %w", err)
	}

	c.client = client
	log.Printf("Connected to IMAP server %s as %s", addr, c.config.Username)
	return nil
}

// ensureConnected reconnects if the connection has been lost.
func (c *IMAPClient) ensureConnected() error {
	if c.client == nil {
		return c.Connect()
	}
	// Try a NOOP to check if the connection is still alive.
	if err := c.client.Noop().Wait(); err != nil {
		log.Printf("IMAP connection lost, reconnecting: %v", err)
		c.client = nil
		return c.Connect()
	}
	return nil
}

// FetchUnseen retrieves all unseen messages from the configured mailbox.
func (c *IMAPClient) FetchUnseen() ([]Email, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	// Select the mailbox.
	if _, err := c.client.Select(c.config.Mailbox, nil).Wait(); err != nil {
		return nil, fmt.Errorf("selecting mailbox %q: %w", c.config.Mailbox, err)
	}

	// Search for unseen messages.
	searchCriteria := &imap.SearchCriteria{
		NotFlag: []imap.Flag{imap.FlagSeen},
	}
	searchData, err := c.client.UIDSearch(searchCriteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("searching for unseen messages: %w", err)
	}

	uids := searchData.AllUIDs()
	if len(uids) == 0 {
		return nil, nil
	}

	// Build a UIDSet from the list of UIDs.
	uidSet := imap.UIDSetNum(uids...)

	// Fetch the messages using Collect() to get buffered data with direct fields.
	fetchOptions := &imap.FetchOptions{
		UID:         true,
		Envelope:    true,
		BodySection: []*imap.FetchItemBodySection{{}},
	}

	messages, err := c.client.Fetch(uidSet, fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("fetching messages: %w", err)
	}

	var emails []Email
	for _, msg := range messages {
		email, err := parseMessageBuffer(msg)
		if err != nil {
			log.Printf("Error parsing message uid=%d: %v", msg.UID, err)
			continue
		}
		emails = append(emails, email)
	}

	return emails, nil
}

// parseMessageBuffer extracts email data from a collected IMAP message buffer.
func parseMessageBuffer(msg *imapclient.FetchMessageBuffer) (Email, error) {
	var email Email

	email.UID = msg.UID

	if msg.Envelope != nil {
		email.Subject = msg.Envelope.Subject
		email.MessageID = msg.Envelope.MessageID
		if len(msg.Envelope.From) > 0 {
			from := msg.Envelope.From[0]
			email.From = fmt.Sprintf("%s@%s", from.Mailbox, from.Host)
		}
	}

	// Extract body from body sections.
	for _, section := range msg.BodySection {
		body, err := extractPlainText(bytes.NewReader(section.Bytes))
		if err != nil {
			log.Printf("Error extracting body for UID %d: %v", email.UID, err)
			continue
		}
		email.Body = body
		break
	}

	return email, nil
}

// extractPlainText reads the message body and extracts the plain text part.
// Falls back to reading the raw body if MIME parsing fails.
func extractPlainText(r io.Reader) (string, error) {
	// Read all data first so we can retry if MIME parsing fails.
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("reading body: %w", err)
	}

	mr, err := mail.CreateReader(bytes.NewReader(data))
	if err != nil {
		// If MIME parsing fails, use the raw body as plain text.
		return string(data), nil
	}

	var plainText string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch part.Header.(type) {
		case *mail.InlineHeader:
			ct := part.Header.Get("Content-Type")
			if ct == "" || strings.HasPrefix(ct, "text/plain") {
				partData, err := io.ReadAll(part.Body)
				if err != nil {
					continue
				}
				plainText = string(partData)
				// Prefer plain text, so return immediately if found.
				return plainText, nil
			}
		}
	}

	if plainText != "" {
		return plainText, nil
	}

	// Fall back to raw body if no text/plain part was found.
	return string(data), nil
}

// MarkSeen flags a message as \Seen by UID.
func (c *IMAPClient) MarkSeen(uid imap.UID) error {
	if err := c.ensureConnected(); err != nil {
		return err
	}

	uidSet := imap.UIDSetNum(uid)

	storeCmd := c.client.Store(uidSet, &imap.StoreFlags{
		Op:     imap.StoreFlagsAdd,
		Silent: true,
		Flags:  []imap.Flag{imap.FlagSeen},
	}, nil)
	if err := storeCmd.Close(); err != nil {
		return fmt.Errorf("marking message %d as seen: %w", uid, err)
	}

	return nil
}

// Close closes the IMAP connection.
func (c *IMAPClient) Close() error {
	if c.client != nil {
		if err := c.client.Logout().Wait(); err != nil {
			return fmt.Errorf("IMAP logout: %w", err)
		}
		c.client = nil
	}
	return nil
}
