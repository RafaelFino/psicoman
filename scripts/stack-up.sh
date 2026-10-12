#!/usr/bin/env bash
# Psicoman — sobe a stack LOCAL de teste (admin + portal) via docker-compose,
# espera ficar saudável e aplica a carga inicial (seed).
#
# Uso:   ./scripts/stack-up.sh [--no-seed]
#        make stack-up
#
# Depois de subir:
#   Admin:  http://localhost:8080/app/
#   Portal: http://localhost:8081/app/
#
# Para limpar tudo:  ./scripts/stack-down.sh  (ou make stack-down)

set -euo pipefail

cd "$(dirname "$0")/.."

c_green=$'\033[0;32m'; c_red=$'\033[0;31m'; c_dim=$'\033[2m'; c_off=$'\033[0m'
info() { printf '%s==>%s %s\n' "$c_green" "$c_off" "$*"; }
die()  { printf '%sERRO:%s %s\n' "$c_red" "$c_off" "$*" >&2; exit 1; }

RUN_SEED=1
[ "${1:-}" = "--no-seed" ] && RUN_SEED=0

command -v docker >/dev/null 2>&1 || die "docker não encontrado."
docker compose version >/dev/null 2>&1 || die "docker compose (v2) não encontrado."

ADMIN_URL="http://localhost:8080"
PORTAL_URL="http://localhost:8081"

info "Construindo e subindo os serviços (admin + portal)..."
docker compose up -d --build

# Espera o healthcheck do compose reportar os dois serviços saudáveis.
wait_healthy() {
  local svc="$1" cid state i
  cid=$(docker compose ps -q "$svc") || true
  [ -n "$cid" ] || die "serviço '$svc' não subiu."
  for i in $(seq 1 60); do
    state=$(docker inspect -f '{{.State.Health.Status}}' "$cid" 2>/dev/null || echo "unknown")
    case "$state" in
      healthy) info "serviço '$svc' saudável."; return 0 ;;
      unhealthy) die "serviço '$svc' ficou unhealthy. Veja: docker compose logs $svc" ;;
    esac
    sleep 1
  done
  die "timeout aguardando '$svc' ficar saudável. Veja: docker compose logs $svc"
}

info "Aguardando serviços ficarem saudáveis..."
wait_healthy admin
wait_healthy portal

if [ "$RUN_SEED" = "1" ]; then
  info "Aplicando carga inicial (seed)..."
  ADMIN_URL="$ADMIN_URL" PORTAL_URL="$PORTAL_URL" ./scripts/seed.sh
else
  info "Pulando o seed (--no-seed)."
fi

echo
info "Stack no ar."
printf '   Admin:   %s/app/\n' "$ADMIN_URL"
printf '   Portal:  %s/app/\n' "$PORTAL_URL"
printf '   Swagger: %s/v1/swagger  ·  %s/v1/swagger\n' "$ADMIN_URL" "$PORTAL_URL"
printf '%s   Logs:    docker compose logs -f   ·   Limpar: ./scripts/stack-down.sh%s\n' "$c_dim" "$c_off"
