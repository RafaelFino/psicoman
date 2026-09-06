#!/usr/bin/env bash
#
# Psicoman — deploy.sh
# Instalador interativo para uma instância (uma VM por terapeuta).
#
# O que faz:
#   1. Verifica pré-requisitos (Go, git, openssl, systemd).
#   2. Pergunta interativamente tudo o que a aplicação precisa.
#   3. Gera o config.yaml (com chave de cifragem AES-256 gerada na hora).
#   4. Compila os dois binários (psicoman-admin, psicoman-portal).
#   5. Cria usuário de serviço, diretórios de dados e units systemd.
#   6. Sobe e habilita os serviços; valida via /readyz.
#
# Uso:
#   sudo ./scripts/deploy.sh
#
# Guia detalhado (Google, Pangolin, DNS): ver docs/deploy.md.

set -euo pipefail

# ------------------------------------------------------------------ helpers
BOLD=$'\e[1m'; DIM=$'\e[2m'; GREEN=$'\e[32m'; YELLOW=$'\e[33m'; RED=$'\e[31m'; RESET=$'\e[0m'

info()  { printf '%s\n' "${GREEN}==>${RESET} $*"; }
warn()  { printf '%s\n' "${YELLOW}!! ${RESET} $*"; }
err()   { printf '%s\n' "${RED}xx ${RESET} $*" >&2; }
die()   { err "$*"; exit 1; }

# ask VAR "Pergunta" "default"
ask() {
  local __var="$1" __prompt="$2" __default="${3:-}" __ans
  if [[ -n "$__default" ]]; then
    read -r -p "${BOLD}${__prompt}${RESET} [${DIM}${__default}${RESET}]: " __ans || true
    __ans="${__ans:-$__default}"
  else
    read -r -p "${BOLD}${__prompt}${RESET}: " __ans || true
  fi
  printf -v "$__var" '%s' "$__ans"
}

# ask_secret VAR "Pergunta"  (não ecoa o valor digitado)
ask_secret() {
  local __var="$1" __prompt="$2" __ans
  read -r -s -p "${BOLD}${__prompt}${RESET}: " __ans || true
  echo
  printf -v "$__var" '%s' "$__ans"
}

# ask_yes_no "Pergunta" default(y|n) -> retorna 0 para sim
ask_yes_no() {
  local __prompt="$1" __default="${2:-y}" __ans
  read -r -p "${BOLD}${__prompt}${RESET} [$( [[ $__default == y ]] && echo 'S/n' || echo 's/N')]: " __ans || true
  __ans="${__ans:-$__default}"
  [[ "$__ans" =~ ^[SsYy]$ ]]
}

require() { command -v "$1" >/dev/null 2>&1 || die "Pré-requisito ausente: $1. Instale antes de continuar."; }

# ------------------------------------------------------------------ start
printf '%s\n' "${BOLD}=== Psicoman — instalação interativa ===${RESET}"
echo "Este script vai configurar uma instância do Psicoman nesta máquina."
echo

if [[ "${EUID}" -ne 0 ]]; then
  warn "Recomenda-se rodar com sudo (para criar usuário de serviço e units systemd)."
  ask_yes_no "Continuar mesmo assim?" n || die "Abortado. Rode: sudo ./scripts/deploy.sh"
fi

info "Verificando pré-requisitos"
require go
require git
require openssl
SYSTEMD=1
command -v systemctl >/dev/null 2>&1 || { warn "systemd não encontrado; pularei a criação de serviços."; SYSTEMD=0; }

# Diretório do repositório (raiz, onde está o go.mod).
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
[[ -f "${REPO_DIR}/go.mod" ]] || die "go.mod não encontrado em ${REPO_DIR}."
info "Repositório: ${REPO_DIR}"

# ------------------------------------------------------------------ perguntas
echo
printf '%s\n' "${BOLD}--- Identidade e paths ---${RESET}"
ask SERVICE_USER   "Usuário de serviço (dono dos dados)"        "psicoman"
ask INSTALL_DIR    "Diretório de instalação dos binários"       "/opt/psicoman"
ask DATA_DIR       "Diretório de dados (SQLite, GED, logs)"      "/var/lib/psicoman"

echo
printf '%s\n' "${BOLD}--- Rede (portas locais atrás do Pangolin) ---${RESET}"
ask ADMIN_HOST     "Host de bind do admin"                       "127.0.0.1"
ask ADMIN_PORT     "Porta do admin"                              "8080"
ask PORTAL_HOST    "Host de bind do portal"                      "127.0.0.1"
ask PORTAL_PORT    "Porta do portal"                             "8081"

echo
printf '%s\n' "${BOLD}--- Autenticação do admin (defense in depth) ---${RESET}"
echo "O terapeuta acessa o admin atrás do Pangolin. O e-mail vem no header do Pangolin"
echo "e um secret é validado pela própria aplicação."
ask ADMIN_EMAIL    "E-mail do terapeuta (admin)"                 ""
[[ -n "${ADMIN_EMAIL}" ]] || die "E-mail do admin é obrigatório."
if ask_yes_no "Gerar um secret admin aleatório?" y; then
  ADMIN_SECRET="$(openssl rand -base64 24)"
  info "Secret admin gerado (anote e configure o mesmo no Pangolin)."
else
  ask_secret ADMIN_SECRET "Secret admin (não será exibido)"
  [[ -n "${ADMIN_SECRET}" ]] || die "Secret admin é obrigatório."
fi
ask EMAIL_HEADER   "Header de e-mail enviado pelo Pangolin"      "X-Pangolin-Email"
ask SECRET_HEADER  "Header de secret enviado pelo Pangolin"      "X-Pangolin-Secret"

echo
printf '%s\n' "${BOLD}--- Integração Google (Calendar/Meet, Gmail, Drive) ---${RESET}"
echo "Crie as credenciais OAuth no Google Cloud antes (ver docs/deploy.md, seção Google)."
echo "Você pode deixar em branco agora e autorizar depois pela API/UI."
ask GOOGLE_CLIENT_ID     "Google OAuth Client ID"                ""
ask_secret GOOGLE_CLIENT_SECRET "Google OAuth Client Secret (não exibido)"
ask GOOGLE_REDIRECT      "Redirect URL (callback do admin)"      "https://admin.seudominio.com.br/v1/admin/google/callback"
ask GOOGLE_CALENDAR      "Calendar ID"                           "primary"
ask GOOGLE_DRIVE_FOLDER  "Pasta lógica de backup no Drive"       "psicoman-backup"

echo
printf '%s\n' "${BOLD}--- Lembretes e rate limiting ---${RESET}"
ask REMINDER_1     "Lembrete 1 (minutos antes do evento)"        "1440"
ask REMINDER_2     "Lembrete 2 (minutos antes do evento)"        "30"
ask RL_RPM         "Rate limit do portal (requisições/minuto)"   "30"
ask RL_BURST       "Rate limit do portal (burst)"                "10"

echo
printf '%s\n' "${BOLD}--- Chave de cifragem (tokens + backup) ---${RESET}"
CRYPTO_KEY="$(openssl rand -base64 32)"
info "Chave AES-256 gerada automaticamente (guardada apenas no config.yaml)."

ask LOG_LEVEL      "Nível de log (debug|info|warn|error)"         "info"

# ------------------------------------------------------------------ resumo
echo
printf '%s\n' "${BOLD}=== Resumo ===${RESET}"
cat <<EOF
  Usuário de serviço : ${SERVICE_USER}
  Instalação         : ${INSTALL_DIR}
  Dados              : ${DATA_DIR}
  Admin              : ${ADMIN_HOST}:${ADMIN_PORT}  (email: ${ADMIN_EMAIL})
  Portal             : ${PORTAL_HOST}:${PORTAL_PORT}
  Google Client ID   : ${GOOGLE_CLIENT_ID:-<vazio, autorizar depois>}
  Redirect URL       : ${GOOGLE_REDIRECT}
  Lembretes (min)    : ${REMINDER_1}, ${REMINDER_2}
  systemd            : $( [[ ${SYSTEMD} -eq 1 ]] && echo sim || echo não )
EOF
echo
ask_yes_no "Confirmar e prosseguir com a instalação?" y || die "Instalação cancelada."

# ------------------------------------------------------------------ usuário e diretórios
if [[ ${EUID} -eq 0 ]]; then
  if ! id "${SERVICE_USER}" >/dev/null 2>&1; then
    info "Criando usuário de serviço ${SERVICE_USER}"
    useradd --system --home "${DATA_DIR}" --shell /usr/sbin/nologin "${SERVICE_USER}" || \
      useradd --system --home "${DATA_DIR}" --shell /sbin/nologin "${SERVICE_USER}"
  fi
fi

info "Criando diretórios"
mkdir -p "${INSTALL_DIR}" "${DATA_DIR}/ged" "${DATA_DIR}/logs"

# ------------------------------------------------------------------ build
info "Compilando binários (make build)"
( cd "${REPO_DIR}" && make build )
install -m 0755 "${REPO_DIR}/bin/psicoman-admin"  "${INSTALL_DIR}/psicoman-admin"
install -m 0755 "${REPO_DIR}/bin/psicoman-portal" "${INSTALL_DIR}/psicoman-portal"

# ------------------------------------------------------------------ config.yaml
CONFIG_PATH="${INSTALL_DIR}/config.yaml"
info "Gerando ${CONFIG_PATH}"
umask 077
cat > "${CONFIG_PATH}" <<EOF
# Psicoman — config gerado por deploy.sh em $(date -Iseconds)
admin:
  host: "${ADMIN_HOST}"
  port: ${ADMIN_PORT}
portal:
  host: "${PORTAL_HOST}"
  port: ${PORTAL_PORT}

paths:
  sqlite: "${DATA_DIR}/psicoman.db"
  ged_root: "${DATA_DIR}/ged"
  log_dir: "${DATA_DIR}/logs"

admin_auth:
  email: "${ADMIN_EMAIL}"
  secret: "${ADMIN_SECRET}"
  email_header: "${EMAIL_HEADER}"
  secret_header: "${SECRET_HEADER}"

google:
  client_id: "${GOOGLE_CLIENT_ID}"
  client_secret: "${GOOGLE_CLIENT_SECRET}"
  redirect_url: "${GOOGLE_REDIRECT}"
  calendar_id: "${GOOGLE_CALENDAR}"
  drive_folder: "${GOOGLE_DRIVE_FOLDER}"
  scopes:
    - "https://www.googleapis.com/auth/calendar"
    - "https://www.googleapis.com/auth/gmail.send"
    - "https://www.googleapis.com/auth/drive.file"

crypto:
  key: "${CRYPTO_KEY}"

log:
  level: "${LOG_LEVEL}"

reminders:
  minutes_before:
    - ${REMINDER_1}
    - ${REMINDER_2}

rate_limit:
  requests_per_minute: ${RL_RPM}
  burst: ${RL_BURST}
EOF
umask 022

if [[ ${EUID} -eq 0 ]]; then
  chown -R "${SERVICE_USER}:${SERVICE_USER}" "${DATA_DIR}" "${INSTALL_DIR}"
  chmod 600 "${CONFIG_PATH}"
fi

# ------------------------------------------------------------------ systemd
if [[ ${SYSTEMD} -eq 1 && ${EUID} -eq 0 ]]; then
  info "Instalando units systemd"
  for surface in admin portal; do
    cat > "/etc/systemd/system/psicoman-${surface}.service" <<EOF
[Unit]
Description=Psicoman ${surface}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/psicoman-${surface} -config ${CONFIG_PATH}
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${DATA_DIR}
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF
  done

  systemctl daemon-reload
  systemctl enable --now psicoman-admin psicoman-portal
  info "Serviços habilitados e iniciados."
else
  warn "Sem systemd (ou sem root). Rode manualmente:"
  echo "  ${INSTALL_DIR}/psicoman-admin  -config ${CONFIG_PATH} &"
  echo "  ${INSTALL_DIR}/psicoman-portal -config ${CONFIG_PATH} &"
fi

# ------------------------------------------------------------------ validação
info "Validando readiness (aguardando os serviços subirem)"
validate() {
  local url="$1" name="$2" i
  for i in $(seq 1 15); do
    if curl -fsS "${url}/readyz" >/dev/null 2>&1; then
      info "${name} pronto (${url}/readyz)"
      return 0
    fi
    sleep 1
  done
  warn "${name} não respondeu /readyz a tempo. Verifique os logs:"
  [[ ${SYSTEMD} -eq 1 ]] && echo "  journalctl -u psicoman-${name} -n 50 --no-pager"
  return 1
}
RC=0
validate "http://${ADMIN_HOST}:${ADMIN_PORT}"   "admin"  || RC=1
validate "http://${PORTAL_HOST}:${PORTAL_PORT}" "portal" || RC=1

echo
if [[ ${RC} -eq 0 ]]; then
  printf '%s\n' "${GREEN}${BOLD}Instalação concluída com sucesso.${RESET}"
else
  printf '%s\n' "${YELLOW}${BOLD}Instalação concluída com avisos — revise os logs.${RESET}"
fi
cat <<EOF

Próximos passos:
  1. Configure o Pangolin apontando para admin (${ADMIN_HOST}:${ADMIN_PORT}) e portal
     (${PORTAL_HOST}:${PORTAL_PORT}). O admin exige controle de acesso; o portal só TLS.
     (ver docs/deploy.md, seção Pangolin)
  2. Autorize o Google: POST https://admin.seudominio.com.br/v1/admin/google/authorize
     e conclua o consentimento (ver docs/deploy.md, seção Google).
  3. Configure seu perfil: PUT /v1/admin/profile.

Config: ${CONFIG_PATH}
Dados : ${DATA_DIR}
EOF
[[ -n "${ADMIN_SECRET:-}" ]] && echo "Secret admin (configure no Pangolin): ${ADMIN_SECRET}"
