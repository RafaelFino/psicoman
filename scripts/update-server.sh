#!/usr/bin/env bash
#
# Psicoman — update-server.sh
# Atualiza a instância a partir do repositório oficial, com segurança.
#
# Fluxo:
#   1. Backup completo ANTES de qualquer mudança (SQLite + GED + config + binários).
#   2. git fetch + checkout da versão alvo (tag mais recente ou a informada).
#   3. Recompila os binários; se a compilação falhar, aborta sem tocar nada em produção.
#   4. Substitui os binários (guardando os anteriores) e reinicia os serviços.
#      As migrations são aplicadas automaticamente no boot dos binários.
#   5. Valida /readyz dos dois serviços. Se falhar, faz ROLLBACK (binários + base) e reinicia.
#
# Uso:
#   sudo ./scripts/update-server.sh [--ref <tag|branch|commit>] [--install-dir DIR] [--data-dir DIR]
#
# Defaults compatíveis com deploy.sh: --install-dir /opt/psicoman  --data-dir /var/lib/psicoman

set -euo pipefail

REPO_URL="https://github.com/rafaelfino/psicoman.git"
INSTALL_DIR="/opt/psicoman"
DATA_DIR="/var/lib/psicoman"
REF=""               # vazio = última tag; senão usa o valor informado
ADMIN_URL=""         # detectado do config se vazio
PORTAL_URL=""

GREEN=$'\e[32m'; YELLOW=$'\e[33m'; RED=$'\e[31m'; BOLD=$'\e[1m'; RESET=$'\e[0m'
info() { printf '%s\n' "${GREEN}==>${RESET} $*"; }
warn() { printf '%s\n' "${YELLOW}!! ${RESET} $*"; }
die()  { printf '%s\n' "${RED}xx ${RESET} $*" >&2; exit 1; }

# ------------------------------------------------------------------ args
while [[ $# -gt 0 ]]; do
  case "$1" in
    --ref)         REF="$2"; shift 2 ;;
    --install-dir) INSTALL_DIR="$2"; shift 2 ;;
    --data-dir)    DATA_DIR="$2"; shift 2 ;;
    --repo)        REPO_URL="$2"; shift 2 ;;
    -h|--help)     grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *)             die "Argumento desconhecido: $1" ;;
  esac
done

command -v go >/dev/null 2>&1 || die "Go não encontrado."
command -v git >/dev/null 2>&1 || die "git não encontrado."
command -v curl >/dev/null 2>&1 || die "curl não encontrado."

CONFIG_PATH="${INSTALL_DIR}/config.yaml"
[[ -f "${CONFIG_PATH}" ]] || die "config.yaml não encontrado em ${INSTALL_DIR}. Rode deploy.sh primeiro."

# Extrai host:port do config para validar /readyz (parse simples de YAML plano).
yaml_get() { awk -v k="$1" '$1==k":"{gsub(/"/,"",$2); print $2}' "${CONFIG_PATH}" | head -1; }
if [[ -z "${ADMIN_URL}" ]]; then
  ah="$(awk '/^admin:/{a=1} a&&/host:/{gsub(/"/,"",$2);print $2;exit}' "${CONFIG_PATH}")"
  ap="$(awk '/^admin:/{a=1} a&&/port:/{print $2;exit}' "${CONFIG_PATH}")"
  ADMIN_URL="http://${ah:-127.0.0.1}:${ap:-8080}"
fi
if [[ -z "${PORTAL_URL}" ]]; then
  ph="$(awk '/^portal:/{p=1} p&&/host:/{gsub(/"/,"",$2);print $2;exit}' "${CONFIG_PATH}")"
  pp="$(awk '/^portal:/{p=1} p&&/port:/{print $2;exit}' "${CONFIG_PATH}")"
  PORTAL_URL="http://${ph:-127.0.0.1}:${pp:-8081}"
fi

DB_PATH="$(awk '/^paths:/{p=1} p&&/sqlite:/{gsub(/"/,"",$2);print $2;exit}' "${CONFIG_PATH}")"
DB_PATH="${DB_PATH:-${DATA_DIR}/psicoman.db}"

TS="$(date +%Y%m%d-%H%M%S)"
BACKUP_ROOT="${DATA_DIR}/updates/${TS}"

# ------------------------------------------------------------------ 1. backup completo
info "Backup completo em ${BACKUP_ROOT}"
mkdir -p "${BACKUP_ROOT}"

# Base SQLite consistente: usa sqlite3 se houver (.backup), senão VACUUM INTO, senão cópia.
if command -v sqlite3 >/dev/null 2>&1 && [[ -f "${DB_PATH}" ]]; then
  sqlite3 "${DB_PATH}" ".backup '${BACKUP_ROOT}/psicoman.db'" \
    || cp -a "${DB_PATH}" "${BACKUP_ROOT}/psicoman.db"
elif [[ -f "${DB_PATH}" ]]; then
  cp -a "${DB_PATH}" "${BACKUP_ROOT}/psicoman.db"
fi

# GED, config e binários atuais.
if [[ -d "${DATA_DIR}/ged" ]]; then
  tar -czf "${BACKUP_ROOT}/ged.tar.gz" -C "${DATA_DIR}" ged
fi
cp -a "${CONFIG_PATH}" "${BACKUP_ROOT}/config.yaml"
[[ -f "${INSTALL_DIR}/psicoman-admin"  ]] && cp -a "${INSTALL_DIR}/psicoman-admin"  "${BACKUP_ROOT}/psicoman-admin.prev"
[[ -f "${INSTALL_DIR}/psicoman-portal" ]] && cp -a "${INSTALL_DIR}/psicoman-portal" "${BACKUP_ROOT}/psicoman-portal.prev"
info "Backup concluído."

# ------------------------------------------------------------------ 2. obter código
SRC_DIR="${DATA_DIR}/src"
if [[ -d "${SRC_DIR}/.git" ]]; then
  info "Atualizando repositório em ${SRC_DIR}"
  git -C "${SRC_DIR}" fetch --all --tags --prune
else
  info "Clonando ${REPO_URL}"
  rm -rf "${SRC_DIR}"
  git clone "${REPO_URL}" "${SRC_DIR}"
fi

if [[ -z "${REF}" ]]; then
  REF="$(git -C "${SRC_DIR}" tag --sort=-v:refname | head -1)"
  [[ -n "${REF}" ]] || REF="origin/HEAD"
  info "Versão alvo (última tag): ${REF}"
fi
git -C "${SRC_DIR}" checkout -q "${REF}"
git -C "${SRC_DIR}" submodule update --init --recursive 2>/dev/null || true
info "Código em: $(git -C "${SRC_DIR}" describe --always --tags 2>/dev/null || echo "${REF}")"

# ------------------------------------------------------------------ 3. build (aborta antes de tocar produção)
info "Compilando nova versão"
if ! ( cd "${SRC_DIR}" && make build ); then
  die "Compilação falhou. Produção intacta. Nada foi alterado."
fi
# Sanidade: testes unitários (não bloqueiam se ausentes, mas alertam).
if ! ( cd "${SRC_DIR}" && make test ); then
  warn "Testes unitários falharam na nova versão."
  read -r -p "Continuar mesmo assim? [s/N]: " ans || true
  [[ "${ans}" =~ ^[SsYy]$ ]] || die "Atualização abortada. Produção intacta."
fi

# ------------------------------------------------------------------ 4. instalar + reiniciar
have_systemd() { command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files 'psicoman-*.service' >/dev/null 2>&1; }

restart_services() {
  if have_systemd; then
    systemctl restart psicoman-admin psicoman-portal
  else
    warn "systemd não detectado — reinicie os processos manualmente."
  fi
}

info "Instalando novos binários (anteriores salvos no backup)"
install -m 0755 "${SRC_DIR}/bin/psicoman-admin"  "${INSTALL_DIR}/psicoman-admin"
install -m 0755 "${SRC_DIR}/bin/psicoman-portal" "${INSTALL_DIR}/psicoman-portal"

info "Reiniciando serviços (migrations rodam no boot)"
restart_services

# ------------------------------------------------------------------ 5. validação + rollback
validate() {
  local url="$1" i
  for i in $(seq 1 20); do
    curl -fsS "${url}/readyz" >/dev/null 2>&1 && return 0
    sleep 1
  done
  return 1
}

info "Validando readiness"
OK=1
validate "${ADMIN_URL}"  || { warn "admin não passou no /readyz"; OK=0; }
validate "${PORTAL_URL}" || { warn "portal não passou no /readyz"; OK=0; }

if [[ ${OK} -eq 1 ]]; then
  printf '%s\n' "${GREEN}${BOLD}Atualização concluída e validada (${REF}).${RESET}"
  info "Backup desta atualização preservado em: ${BACKUP_ROOT}"
  exit 0
fi

# ------------------------------------------------------------------ ROLLBACK
warn "Validação falhou — iniciando ROLLBACK."
if [[ -f "${BACKUP_ROOT}/psicoman-admin.prev" ]]; then
  install -m 0755 "${BACKUP_ROOT}/psicoman-admin.prev"  "${INSTALL_DIR}/psicoman-admin"
fi
if [[ -f "${BACKUP_ROOT}/psicoman-portal.prev" ]]; then
  install -m 0755 "${BACKUP_ROOT}/psicoman-portal.prev" "${INSTALL_DIR}/psicoman-portal"
fi
# Restaura a base apenas se a migration da nova versão pode ter alterado o schema.
if [[ -f "${BACKUP_ROOT}/psicoman.db" ]]; then
  warn "Restaurando a base do backup pré-atualização."
  cp -a "${BACKUP_ROOT}/psicoman.db" "${DB_PATH}"
fi
restart_services

if validate "${ADMIN_URL}" && validate "${PORTAL_URL}"; then
  die "Rollback concluído: versão anterior restaurada e validada. Investigue a falha da nova versão."
else
  die "Rollback executado, mas os serviços ainda não respondem. Backup em ${BACKUP_ROOT}. Intervenção manual necessária."
fi
