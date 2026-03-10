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

2. **Configure** (pick one)

Option A — env vars only (no config file needed):
```bash
export AGENT="researcher"
export IMAP_HOST="imap.gmail.com"
export IMAP_USERNAME="agent@yourdomain.com"
export IMAP_PASSWORD="your-app-password"
export SMTP_HOST="smtp.gmail.com"
export SMTP_USERNAME="agent@yourdomain.com"
export SMTP_PASSWORD="your-app-password"
export SMTP_FROM="agent@yourdomain.com"
```

Option B — config file:
```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your IMAP/SMTP credentials and agent name
```

3. **Run**

```bash
./agentfile-email-bridge
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `config.yaml` | Path to configuration file (optional if using env vars) |
| `--agent` | _(from config)_ | Override agent name from config/env |

## Configuration

All settings can come from a YAML file, environment variables, or both. Precedence (highest wins): **CLI flags > env vars > YAML file > defaults**.

The config file is optional. If no `config.yaml` exists and no `--config` is passed, the bridge configures entirely from env vars.

See [`config.example.yaml`](config.example.yaml) for a fully commented example.

### Environment Variables

| Env Var | Required | Default | Description |
|---------|----------|---------|-------------|
| `AGENT` | **yes** | | Agent name to trigger (e.g. `researcher`) |
| `AGENTFILE_URL` | no | `http://localhost:3000` | Agentfile REST API base URL |
| `MAX_CONCURRENT` | no | `5` | Max emails processed in parallel |
| `IMAP_HOST` | **yes** | | IMAP server hostname |
| `IMAP_PORT` | no | `993` | IMAP server port |
| `IMAP_USERNAME` | **yes** | | IMAP login username |
| `IMAP_PASSWORD` | **yes** | | IMAP login password |
| `IMAP_TLS` | no | `true` | Use TLS (`true`/`false`) |
| `IMAP_MAILBOX` | no | `INBOX` | IMAP folder to watch |
| `IMAP_POLL_INTERVAL` | no | `30s` | Poll frequency (Go duration) |
| `SMTP_HOST` | **yes** | | SMTP server hostname |
| `SMTP_PORT` | no | `587` | SMTP server port |
| `SMTP_USERNAME` | **yes** | | SMTP login username |
| `SMTP_PASSWORD` | **yes** | | SMTP login password |
| `SMTP_FROM` | **yes** | | From address on outgoing replies |
| `AGENTFILE_TIMEOUT` | no | `5m` | HTTP timeout for agent calls (Go duration) |

## How It Works

1. The bridge polls the configured IMAP mailbox for unseen messages
2. For each unseen email, it extracts the plain text body
3. It sends the body to the agentfile agent via `POST /api/v1/agents/{name}/run`
4. The agent's response is sent back as an email reply (with proper threading headers)
5. The original email is marked as seen

**Error handling:** If the agent call or reply fails, the email is left as unseen and will be retried on the next poll cycle.

## Docker

With env vars (no config file):
```bash
docker build -t agentfile-email-bridge .

docker run \
  -e AGENT=researcher \
  -e IMAP_HOST=imap.gmail.com \
  -e IMAP_USERNAME=agent@yourdomain.com \
  -e IMAP_PASSWORD=your-app-password \
  -e SMTP_HOST=smtp.gmail.com \
  -e SMTP_USERNAME=agent@yourdomain.com \
  -e SMTP_PASSWORD=your-app-password \
  -e SMTP_FROM=agent@yourdomain.com \
  agentfile-email-bridge
```

With a config file:
```bash
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
