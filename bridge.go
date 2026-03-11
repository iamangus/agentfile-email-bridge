package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/emersion/go-imap/v2"
)

// Bridge orchestrates the poll-call-reply loop.
type Bridge struct {
	config    *Config
	imap      *IMAPClient
	smtp      *SMTPClient
	agentfile *AgentfileClient

	// inFlight tracks UIDs currently being processed by a goroutine.
	// The next poll skips any UID already present here, preventing
	// duplicate agent calls for slow-running requests.
	mu       sync.Mutex
	inFlight map[imap.UID]struct{}

	// sem is a counting semaphore that caps the number of emails
	// processed concurrently (sized to config.MaxConcurrent).
	sem chan struct{}

	// wg tracks in-flight goroutines so Run() can wait for them
	// during graceful shutdown.
	wg sync.WaitGroup
}

// NewBridge creates a new Bridge with all clients wired up.
func NewBridge(config *Config, imapClient *IMAPClient, smtpClient *SMTPClient, agentfileClient *AgentfileClient) *Bridge {
	return &Bridge{
		config:    config,
		imap:      imapClient,
		smtp:      smtpClient,
		agentfile: agentfileClient,
		inFlight:  make(map[imap.UID]struct{}),
		sem:       make(chan struct{}, config.MaxConcurrent),
	}
}

// Run starts the main polling loop. It blocks until the context is cancelled
// and all in-flight goroutines have finished.
func (b *Bridge) Run(ctx context.Context) error {
	log.Printf("Starting bridge: polling every %s, agent=%q, agentfile=%s, max_concurrent=%d",
		b.config.IMAP.PollInterval, b.config.Agent, b.config.AgentfileURL, b.config.MaxConcurrent)

	// Run once immediately before entering the loop.
	b.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Bridge shutting down, waiting for in-flight emails to finish...")
			b.wg.Wait()
			log.Println("All in-flight emails finished")
			return nil
		case <-time.After(b.config.IMAP.PollInterval):
			b.poll(ctx)
		}
	}
}

// poll fetches unseen emails and dispatches a goroutine for each new one.
// Emails already being processed (in the inFlight set) are skipped.
func (b *Bridge) poll(ctx context.Context) {
	emails, err := b.imap.FetchUnseen()
	if err != nil {
		log.Printf("Error fetching unseen emails: %v", err)
		return
	}

	if len(emails) == 0 {
		return
	}

	log.Printf("Found %d unseen email(s)", len(emails))

	for _, email := range emails {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Skip emails already being processed.
		b.mu.Lock()
		if _, ok := b.inFlight[email.UID]; ok {
			b.mu.Unlock()
			log.Printf("Skipping uid=%d: already in-flight", email.UID)
			continue
		}
		b.inFlight[email.UID] = struct{}{}
		b.mu.Unlock()

		b.wg.Add(1)

		// Capture loop variable for the goroutine.
		email := email

		go func() {
			defer b.wg.Done()
			defer func() {
				b.mu.Lock()
				delete(b.inFlight, email.UID)
				b.mu.Unlock()
			}()

			// Acquire semaphore slot (blocks if at capacity).
			select {
			case b.sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-b.sem }()

			b.processEmail(ctx, email)
		}()
	}
}

// processEmail handles a single email: calls the agent and sends the reply.
// On failure, the email is left UNSEEN so it will be retried on the next poll.
func (b *Bridge) processEmail(ctx context.Context, email Email) {
	log.Printf("Processing email from=%s subject=%q uid=%d", email.From, email.Subject, email.UID)

	if email.Body == "" {
		log.Printf("Skipping email uid=%d: empty body", email.UID)
		// Mark as seen to avoid retrying empty emails forever.
		if err := b.imap.MarkSeen(email.UID); err != nil {
			log.Printf("Error marking empty email uid=%d as seen: %v", email.UID, err)
		}
		return
	}

	// Call the agentfile agent with a formatted message that includes metadata.
	message := formatAgentMessage(email)
	response, err := b.agentfile.RunAgent(ctx, b.config.Agent, message)
	if err != nil {
		log.Printf("Error calling agent %q for uid=%d: %v", b.config.Agent, email.UID, err)
		// Leave UNSEEN so it will be retried on next poll.
		return
	}

	// Send the reply.
	if err := b.smtp.SendReply(email.From, email.Subject, email.MessageID, response); err != nil {
		log.Printf("Error sending reply for uid=%d: %v", email.UID, err)
		// Leave UNSEEN so it will be retried on next poll.
		return
	}

	// Mark the email as seen.
	if err := b.imap.MarkSeen(email.UID); err != nil {
		log.Printf("Error marking uid=%d as seen: %v", email.UID, err)
		// The reply was already sent, so this is not critical.
		// The email may be processed again on the next poll, but the duplicate
		// reply is preferable to losing the response entirely.
		return
	}

	log.Printf("Successfully processed email uid=%d from=%s", email.UID, email.From)
}

// formatAgentMessage builds the message string sent to the agent, wrapping the
// email body with a structured preamble that includes sender and subject metadata.
func formatAgentMessage(email Email) string {
	return fmt.Sprintf(`You have received the following research request via email!
Your response will be sent back to the sender as a plain text email, so do not use markdown formatting in your response.

From: %s
Subject: %s

---

%s`, email.From, email.Subject, email.Body)
}
