#!/usr/bin/env bash
# Interactive quickstart: prompt for .env values, then docker compose build + up (local stack).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

die() {
  echo "Error: $*" >&2
  exit 1
}

command -v docker >/dev/null 2>&1 || die "docker not found. Install Docker Desktop or the Docker Engine CLI."

if ! docker compose version >/dev/null 2>&1; then
  die "docker compose not available. Use Docker Compose V2 (docker compose)."
fi

rand_hex() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 16
  else
    # Fallback: weaker, but avoids requiring openssl
    printf '%x' "$RANDOM$RANDOM$RANDOM$RANDOM" | head -c 32
    echo
  fi
}

rand_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 32 | tr -d '\n'
  else
    rand_hex
    rand_hex
  fi
}

read_default() {
  local prompt="$1" default="$2"
  local input
  read -r -p "${prompt} [${default}]: " input || true
  if [[ -z "${input:-}" ]]; then
    printf '%s' "$default"
  else
    printf '%s' "$input"
  fi
}

read_optional_empty() {
  local prompt="$1"
  local input
  read -r -p "${prompt} (leave empty to skip): " input || true
  printf '%s' "${input:-}"
}

write_env_file() {
  local DATABASE_URL
  DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@db:5432/${POSTGRES_DB}?sslmode=disable"

  : > .env
  {
    printf '%s\n' '# Database'
    printf 'POSTGRES_USER=%s\n' "$POSTGRES_USER"
    printf 'POSTGRES_PASSWORD=%s\n' "$POSTGRES_PASSWORD"
    printf 'POSTGRES_DB=%s\n' "$POSTGRES_DB"
    printf 'DATABASE_URL=%s\n' "$DATABASE_URL"
    printf '%s\n' ''
    printf '%s\n' '# IBKR (use IBKR_MODE=paper or live when a real gateway is available)'
    printf 'IBKR_GATEWAY_HOST=ib-gateway\n'
    printf 'IBKR_GATEWAY_PORT=4001\n'
    printf 'IBKR_MODE=%s\n' "$IBKR_MODE"
    printf '%s\n' ''
    printf '%s\n' '# Auth (portfolio-api)'
    printf 'AUTH_USERNAME=%s\n' "$AUTH_USERNAME"
    printf 'AUTH_PASSWORD=%s\n' "$AUTH_PASSWORD"
    printf 'AUTH_SECRET=%s\n' "$AUTH_SECRET"
    printf '%s\n' ''
    printf '%s\n' '# Internal API (ingest → portfolio-api)'
    printf 'INTERNAL_API_KEY=%s\n' "$INTERNAL_API_KEY"
    printf '%s\n' ''
    printf '%s\n' '# Ingest'
    printf 'INGEST_INTERVAL=10m\n'
    printf 'PORTFOLIO_API_URL=http://portfolio-api:8080\n'
    printf '%s\n' ''
    printf '%s\n' '# Signals'
    printf 'SIGNAL_RULES_PATH=./config/signals.yaml\n'
    printf 'SIGNAL_COOLDOWN=1h\n'
    printf 'SIGNAL_INTERVAL=5m\n'
    printf 'SIGNAL_DEDUP_PATH=./data/signal-dedup.json\n'
    printf '%s\n' ''
    printf '%s\n' '# IBKR Client Portal (paper/live; optional)'
    printf 'IBKR_CLIENT_PORTAL_PORT=5000\n'
    printf '%s\n' ''
    printf '%s\n' '# Discord'
    if [[ -n "${DISCORD_WEBHOOK_URL:-}" ]]; then
      printf 'DISCORD_WEBHOOK_URL=%s\n' "$DISCORD_WEBHOOK_URL"
    else
      printf 'DISCORD_WEBHOOK_URL=\n'
    fi
    printf '%s\n' ''
    printf '%s\n' '# portfolio-api CORS (only if you set NEXT_PUBLIC_API_URL for direct browser → :8080)'
    printf 'CORS_ALLOWED_ORIGINS=%s\n' "$CORS_ALLOWED_ORIGINS"
    printf '%s\n' ''
    printf '%s\n' '# Web (Next.js rewrites /api-go to portfolio-api — leave NEXT_PUBLIC unset for LAN-friendly default)'
    printf 'PORTFOLIO_API_INTERNAL_URL=http://portfolio-api:8080\n'
  } >> .env
}

echo ""
echo "=== morgans-d-stonks quickstart ==="
echo ""

if [[ ! -f .env.example ]]; then
  die ".env.example is missing from the repo root."
fi

CONFIGURE=true
if [[ -f .env ]]; then
  read -r -p ".env already exists. Reconfigure it with new prompts? [y/N] " overwrite || true
  case "${overwrite:-}" in
    y|Y|yes|YES) CONFIGURE=true ;;
    *) CONFIGURE=false ;;
  esac
fi

if [[ "$CONFIGURE" == true ]]; then
  echo ""
  echo "Enter values (press Enter to accept the default in brackets)."
  echo "Generated secrets avoid # in passwords so the .env file parses correctly."
  echo ""

  default_pg_pass="$(rand_hex)"
  POSTGRES_USER="$(read_default "Postgres username" "portfolio")"
  POSTGRES_PASSWORD="$(read_default "Postgres password" "$default_pg_pass")"
  POSTGRES_DB="$(read_default "Postgres database name" "portfolio")"

  AUTH_USERNAME="$(read_default "Dashboard / API login username" "admin")"
  default_auth_pass="$(rand_hex)"
  AUTH_PASSWORD="$(read_default "Dashboard / API login password" "$default_auth_pass")"
  echo -n "API session secret (min ~32 chars; press Enter to generate random): "
  read -r auth_secret_in || true
  if [[ -z "${auth_secret_in:-}" ]]; then
    AUTH_SECRET="$(rand_secret)"
    echo "(generated)"
  else
    AUTH_SECRET="$auth_secret_in"
  fi

  echo -n "Internal API key for ingest/signals (press Enter to generate random): "
  read -r internal_in || true
  if [[ -z "${internal_in:-}" ]]; then
    INTERNAL_API_KEY="$(rand_hex)$(rand_hex)"
    echo "(generated)"
  else
    INTERNAL_API_KEY="$internal_in"
  fi

  DISCORD_WEBHOOK_URL="$(read_optional_empty "Discord webhook URL for signals")"

  echo ""
  echo "IBKR: 1) mock (no live gateway, recommended for local Docker)"
  echo "      2) paper"
  read -r -p "Choice [1]: " ibkr_choice || true
  case "${ibkr_choice:-1}" in
    2) IBKR_MODE="paper" ;;
    *) IBKR_MODE="mock" ;;
  esac

  CORS_ALLOWED_ORIGINS="http://localhost:3000,http://127.0.0.1:3000"

  write_env_file
  echo ""
  echo "Wrote .env (keep it secret; do not commit)."
else
  echo "Using existing .env."
fi

echo ""
read -r -p "Press Enter to build and start the stack locally..." _

echo ""
echo "Running: docker compose build"
docker compose build

echo ""
echo "Running: docker compose up -d"
docker compose up -d

echo ""
echo "Stack is running locally."
echo "  Web:  http://localhost:3000"
echo "  API:  http://localhost:8080/api/health"
echo ""
echo "Logs:    docker compose logs -f"
echo "Stop:    docker compose down"
echo ""
