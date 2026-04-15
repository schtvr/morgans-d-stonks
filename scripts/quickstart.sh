#!/usr/bin/env bash
# Interactive quickstart: .env setup guidance, then docker compose build.
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

echo ""
echo "=== morgans-d-stonks quickstart ==="
echo ""

if [[ ! -f .env ]]; then
  if [[ ! -f .env.example ]]; then
    die ".env.example is missing from the repo root."
  fi
  cp .env.example .env
  echo "Created .env from .env.example"
else
  echo ".env already exists (left unchanged)."
fi

echo ""
echo "Review and adjust .env as needed:"
echo "  • DATABASE_URL — must match POSTGRES_* if you change DB credentials"
echo "  • INTERNAL_API_KEY — shared secret for ingest/signals → portfolio-api"
echo "  • AUTH_USERNAME / AUTH_PASSWORD / AUTH_SECRET — API and dashboard login"
echo "  • DISCORD_WEBHOOK_URL — optional, for signals notifications"
echo "  • IBKR_MODE=mock — good default until a real IB Gateway is configured"
echo "  • NEXT_PUBLIC_API_URL — browser calls API; use http://localhost:8080 for local web"
echo ""

if [[ -n "${EDITOR:-}" ]]; then
  read -r -p "Open .env in \$EDITOR (${EDITOR}) now? [y/N] " ans || true
  case "${ans:-}" in
    y|Y|yes|YES) "$EDITOR" .env ;;
  esac
else
  echo "Tip: set EDITOR (e.g. export EDITOR=nano) to open .env from this script next time."
fi

echo ""
read -r -p "Press Enter when you are ready to build Docker images..." _

echo ""
echo "Running: docker compose build"
docker compose build

echo ""
echo "Done. Start the stack with: docker compose up"
echo "  Web:  http://localhost:3000"
echo "  API:  http://localhost:8080/api/health"
echo ""
