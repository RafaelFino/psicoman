#!/usr/bin/env bash
# Psicoman — derruba a stack LOCAL de teste e LIMPA TUDO, sem deixar lixo:
# containers, rede, volume de dados (banco/GED/logs) e a imagem local de dev.
#
# Uso:   ./scripts/stack-down.sh [--keep-image]
#        make stack-down
#
# Por padrão remove a imagem psicoman-local:dev também. Use --keep-image para
# preservá-la (acelera o próximo stack-up).

set -euo pipefail

cd "$(dirname "$0")/.."

c_green=$'\033[0;32m'; c_dim=$'\033[2m'; c_off=$'\033[0m'
info() { printf '%s==>%s %s\n' "$c_green" "$c_off" "$*"; }

KEEP_IMAGE=0
[ "${1:-}" = "--keep-image" ] && KEEP_IMAGE=1

if ! command -v docker >/dev/null 2>&1; then
  echo "docker não encontrado; nada a limpar." >&2
  exit 0
fi

# down remove containers, rede e o volume nomeado (-v). --remove-orphans pega
# qualquer serviço que tenha sido renomeado/removido do compose.
if [ "$KEEP_IMAGE" = "1" ]; then
  info "Derrubando serviços e removendo volume de dados (mantendo a imagem)..."
  docker compose down -v --remove-orphans
else
  info "Derrubando serviços, volume de dados e imagem local..."
  docker compose down -v --rmi local --remove-orphans
fi

# Garantia extra: remove o volume de dados nomeado caso tenha ficado órfão
# (ex.: criado por uma execução anterior com outro nome de projeto).
if docker volume inspect psicoman-local-data >/dev/null 2>&1; then
  docker volume rm psicoman-local-data >/dev/null 2>&1 || true
  info "volume psicoman-local-data removido."
fi

# Garantia extra: remove a imagem de dev se sobrou (ex.: --keep-image anterior).
if [ "$KEEP_IMAGE" = "0" ] && docker image inspect psicoman-local:dev >/dev/null 2>&1; then
  docker image rm psicoman-local:dev >/dev/null 2>&1 || true
  info "imagem psicoman-local:dev removida."
fi

info "Limpeza concluída."
printf '%s   Nada da stack local permanece (containers, rede, volume e imagem).%s\n' "$c_dim" "$c_off"
