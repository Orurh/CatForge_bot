# Temporary Codex subscription provider

This local beta mode uses the host owner's ChatGPT/Codex subscription, not API
credits. It shares subscription limits with development work. Model:
`gpt-5.6-luna`, reasoning effort `none`. Even a short request currently uses
roughly 6,000 input tokens in Codex scaffolding. The API provider's 180-token
limit does not bound Codex generation; the bridge asks for 300 characters,
validates output, kills requests after 25 seconds and allows 100 attempts per
UTC day. Failed attempts count too. The quota survives service restarts.

`scripts/codex_bridge.py` runs under the owner's user systemd service
`catforge-codex-bridge.service`. Codex must already be logged in via ChatGPT.
The service uses the installed CLI path; update the unit if the extension path
changes. It uses an empty temporary working directory, ignores user config,
disables shell/apps/plugins/hooks/browser/computer/image tools and does not
pass bot secrets to Codex. No prompt or reply bodies are logged by the bridge.
The CLI still owns its normal account state and credential refresh.

The Unix socket `.codex-bridge/bridge.sock` is owner/group-accessible only.
`CODEX_BRIDGE_GID` in `.env` must match the host owner's group. The bot mounts
the socket directory read-only; credentials remain on the host. Do not expose
this bridge publicly. Only one generation runs at a time; concurrent requests,
quota exhaustion and provider failures use the existing procedural fallback.

The dev Compose file now mounts the bridge socket directly and `.env` selects
`AI_PROVIDER=codex`, `AI_MODEL=gpt-5.6-luna`, `AI_TIMEOUT=30s`. The Codex overlay
is optional; ordinary dev + monitoring updates retain the provider. The host
bridge group must match `MONITORING_BACKUP_GID` when monitoring is used.

Run/update the existing beta stack:

```bash
docker compose -f docker-compose.dev.yml -f docker-compose.monitoring.yml build bot
docker compose -f docker-compose.dev.yml -f docker-compose.monitoring.yml up -d --no-deps bot
systemctl --user status catforge-codex-bridge.service
```

The Codex overlay selects `AI_PROVIDER=codex`, clears `AI_API_KEY` in the bot
container, and sets `AI_TIMEOUT=30s`. PostgreSQL/engine are not replaced.
Set `AI_PROVIDER=procedural` in `.env`, omit the Codex overlay and recreate bot
to return to template replies. Stop the bridge using
`systemctl --user disable --now catforge-codex-bridge.service`.

Validation: Python quota/restart and process-timeout tests; Go Unix-socket
provider test; targeted Go race tests; real subscription-generated reply through
the bridge. No Telegram message is sent by those checks.

Official references: https://learn.chatgpt.com/docs/non-interactive-mode and
https://learn.chatgpt.com/docs/auth .
