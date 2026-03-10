# agentfile-email-bridge

A standalone Go service that polls an IMAP mailbox for new emails, forwards the email body to an [agentfile](https://github.com/anomalyco/agentfile) agent via its REST API, and sends the agent's response back as an email reply via SMTP.

## Architecture

```
┌─────────────┐     IMAP poll      ┌──────────────────────┐    HTTP POST     ┌────────────┐
│  Mailbox     │ ──────────────────>│  agentfile-email-    │ ───────────────> │  agentfile │
│  (Gmail,     │                    │  bridge              │ <─────────────── │  :3000     │
│   etc.)      │ <──────────────────│                      │    JSON response │            │
│              │     SMTP reply     │                      │                  │            │
└─────────────┘                    └──────────────────────┘                  └────────────┘
```

## Quick Start

1. **Build**

```bash
go build -o agentfile-email-bridge .
```

2. **Configure**

```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your IMAP/SMTP credentials and agent name
```

3. **Set secrets via environment variables**

```bash
export EMAIL_PASSWORD="your-app-password"
```

4. **Run**

```bash
./agentfile-email-bridge --config config.yaml
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `config.yaml` | Path to configuration file |
| `--agent` | _(from config)_ | Override agent name from config |

## Configuration

See [`config.example.yaml`](config.example.yaml) for a fully commented example. All fields supporting `${ENV_VAR}` syntax will have environment variables expanded at load time.

### Key Settings

- **`agent`** — The agentfile agent to route emails to (e.g. `researcher`, `assistant`)
- **`agentfile_url`** — Base URL of the agentfile REST API
- **`imap.poll_interval`** — How often to check for new mail (default `30s`)
- **`agentfile.timeout`** — HTTP timeout for agent calls (default `5m`)

## How It Works

1. The bridge polls the configured IMAP mailbox for unseen messages
2. For each unseen email, it extracts the plain text body
3. It sends the body to the agentfile agent via `POST /api/v1/agents/{name}/run`
4. The agent's response is sent back as an email reply (with proper threading headers)
5. The original email is marked as seen

**Error handling:** If the agent call or reply fails, the email is left as unseen and will be retried on the next poll cycle.

## Docker

```bash
docker build -t agentfile-email-bridge .

docker run -v ./config.yaml:/etc/agentfile-email-bridge/config.yaml \
  -e EMAIL_PASSWORD="your-password" \
  agentfile-email-bridge
```

## Gmail Setup

For Gmail, you need to use an [App Password](https://support.google.com/accounts/answer/185833):

1. Enable 2-Step Verification on your Google account
2. Generate an App Password at https://myaccount.google.com/apppasswords
3. Use the generated password in your config (via `${EMAIL_PASSWORD}`)

## Dependencies

- [`go-imap/v2`](https://github.com/emersion/go-imap) — IMAP client
- [`go-message`](https://github.com/emersion/go-message) — Email MIME parsing
- [`yaml.v3`](https://github.com/go-yaml/yaml) — YAML configuration
- Go stdlib `net/smtp` — SMTP with STARTTLS
