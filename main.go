package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	agentOverride := flag.String("agent", "", "override agent name from config")
	flag.Parse()

	// Load configuration.
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// CLI flag overrides config file.
	if *agentOverride != "" {
		cfg.Agent = *agentOverride
	}

	// Create clients.
	imapClient := NewIMAPClient(cfg.IMAP)
	smtpClient := NewSMTPClient(cfg.SMTP)
	agentfileClient := NewAgentfileClient(cfg.AgentfileURL, cfg.Agentfile.Timeout)

	// Connect to IMAP on startup to fail fast if credentials are wrong.
	if err := imapClient.Connect(); err != nil {
		log.Fatalf("Failed to connect to IMAP: %v", err)
	}
	defer imapClient.Close()

	// Create the bridge and start the poll loop.
	bridge := NewBridge(cfg, imapClient, smtpClient, agentfileClient)

	// Set up graceful shutdown on SIGINT/SIGTERM.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	log.Println("agentfile-email-bridge starting")
	if err := bridge.Run(ctx); err != nil {
		log.Fatalf("Bridge error: %v", err)
	}
	log.Println("agentfile-email-bridge stopped")
}
