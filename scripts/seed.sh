#!/usr/bin/env bash
# Psicoman — carga inicial (seed) para testar a navegabilidade do sistema.
#
# Popula, via API, um conjunto coerente de dados de demonstração:
#   - origens (canais de aquisição)
#   - locais de atendimento + janelas de disponibilidade
#   - pacientes já aprovados (cadastrados "pelo terapeuta")
#   - 1 auto-cadastro pelo portal -> nasce PENDENTE (exercita a fila de aprovação)
#   - planos (um de cada tipo relevante)
#   - categorias e itens de custo (base do relatório de ROI)
#   - sessões (status "solicitada" — não dispara integração Google)
#
# Pré-requisito: a stack precisa estar NO AR em modo dev (make stack-up).
# Em modo dev a autenticação do admin é desligada e o login do portal aceita
# qualquer e-mail, então o seed roda sem credenciais reais.
#
# Uso:
#   ADMIN_URL=http://localhost:8080 PORTAL_URL=http://localhost:8081 ./scripts/seed.sh
# (os defaults já apontam para a stack local)
#
# NOTA DE IMPLEMENTAÇÃO: todas as mensagens de log vão para STDERR. Só o id de um
# recurso recém-criado é escrito em STDOUT, para poder ser capturado com $(...).

set -euo pipefail

ADMIN_URL="${ADMIN_URL:-http://localhost:8080}"
PORTAL_URL="${PORTAL_URL:-http://localhost:8081}"
ADMIN="${ADMIN_URL%/}/v1/admin"
PORTAL="${PORTAL_URL%/}/v1/portal"

# --- utilidades de saída (SEMPRE em stderr) --------------------------------
c_green=$'\033[0;32m'; c_yellow=$'\033[0;33m'; c_red=$'\033[0;31m'; c_dim=$'\033[2m'; c_off=$'\033[0m'
info()  { printf '%s==>%s %s\n' "$c_green" "$c_off" "$*" >&2; }
warn()  { printf '%s!!%s %s\n'  "$c_yellow" "$c_off" "$*" >&2; }
die()   { printf '%sERRO:%s %s\n' "$c_red" "$c_off" "$*" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || die "curl não encontrado."
HAVE_JQ=0; command -v jq >/dev/null 2>&1 && HAVE_JQ=1

# json_field <json> <campo>: extrai data.{campo} (string) do envelope.
# Usa jq quando disponível; senão, um extrator simples por regex.
json_field() {
  local json="$1" field="$2"
  if [ "$HAVE_JQ" = "1" ]; then
    printf '%s' "$json" | jq -r ".data.${field} // empty" 2>/dev/null
    return
  fi
  printf '%s' "$json" | sed -n "s/.*\"${field}\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" | head -n1
}

# req <METHOD> <URL> [json_body]: faz a chamada e ecoa o corpo da resposta em
# STDOUT. Em erro HTTP (>=400) avisa (stderr) e devolve o corpo mesmo assim.
req() {
  local method="$1" url="$2" body="${3:-}"
  local out code payload
  if [ -n "$body" ]; then
    out=$(curl -sS -m 20 -w $'\n%{http_code}' -X "$method" "$url" \
      -H 'Content-Type: application/json' -H 'Accept: application/json' \
      --data "$body") || { warn "falha de rede em $method $url"; return 1; }
  else
    out=$(curl -sS -m 20 -w $'\n%{http_code}' -X "$method" "$url" \
      -H 'Accept: application/json') || { warn "falha de rede em $method $url"; return 1; }
  fi
  code="${out##*$'\n'}"
  payload="${out%$'\n'*}"
  if [ "$code" -ge 400 ]; then
    warn "$method ${url##*/v1/} -> HTTP $code ${c_dim}${payload}${c_off}"
    printf '%s' "$payload"
    return 1
  fi
  printf '%s' "$payload"
}

# create_id <METHOD> <URL> <body> <descrição>: cria e ecoa SÓ o data.id (stdout).
# O log vai para stderr, então a captura $(...) recebe apenas o id.
create_id() {
  local method="$1" url="$2" body="$3" desc="$4" resp id
  resp=$(req "$method" "$url" "$body") || true
  id=$(json_field "$resp" "id")
  if [ -n "$id" ]; then
    info "$desc ${c_dim}($id)${c_off}"
  else
    warn "$desc — sem id na resposta (pode já existir)."
  fi
  printf '%s' "$id"
}

# post <URL> <body> <descrição>: cria sem precisar do id (log em stderr).
post() {
  local url="$1" body="$2" desc="$3"
  if req POST "$url" "$body" >/dev/null; then
    info "$desc"
  fi
}

# iso <dias> <HH:MM>: devolve um RFC3339 em dias a partir de hoje no horário dado.
iso() {
  local days="$1" hhmm="$2"
  date -u -d "+${days} day ${hhmm}" '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null \
    || date -u -v+"${days}"d '+%Y-%m-%dT'"${hhmm}"':00Z'
}

# --- espera a API ----------------------------------------------------------
info "Aguardando a API do admin em ${ADMIN_URL} ..."
for i in $(seq 1 30); do
  if curl -sS -m 3 -o /dev/null "${ADMIN_URL}/healthz" 2>/dev/null; then break; fi
  [ "$i" = "30" ] && die "admin não respondeu em ${ADMIN_URL}/healthz"
  sleep 1
done
info "API no ar. Iniciando carga de dados.${c_dim} (jq=$HAVE_JQ)${c_off}"

# --- 1) Origens ------------------------------------------------------------
info "Origens (canais de aquisição)"
ORIGIN_INDICACAO=$(create_id POST "$ADMIN/origins" '{"name":"Indicação"}'   "origem: Indicação")
ORIGIN_DOCTORALIA=$(create_id POST "$ADMIN/origins" '{"name":"Doctoralia"}' "origem: Doctoralia")
ORIGIN_INSTAGRAM=$(create_id POST "$ADMIN/origins" '{"name":"Instagram"}'   "origem: Instagram")

# --- 2) Locais + disponibilidade ------------------------------------------
info "Locais de atendimento + disponibilidade"
LOC_CENTRO=$(create_id POST "$ADMIN/locations" \
  '{"name":"Consultório Centro","address":"Rua Augusta, 1000 - São Paulo/SP","modality":"presencial","cost_amount":120000,"cost_period":"mensal"}' \
  "local: Consultório Centro (presencial)")
LOC_ONLINE=$(create_id POST "$ADMIN/locations" \
  '{"name":"Atendimento Online","address":"","modality":"online","cost_amount":5000,"cost_period":"mensal"}' \
  "local: Atendimento Online")

# Disponibilidade: segunda a sexta (weekday 1..5), manhã e tarde no Centro.
if [ -n "$LOC_CENTRO" ]; then
  for wd in 1 2 3 4 5; do
    req POST "$ADMIN/locations/$LOC_CENTRO/availability" \
      "{\"weekday\":$wd,\"start_time\":\"09:00\",\"end_time\":\"12:00\",\"capacity\":1}" >/dev/null || true
    req POST "$ADMIN/locations/$LOC_CENTRO/availability" \
      "{\"weekday\":$wd,\"start_time\":\"14:00\",\"end_time\":\"18:00\",\"capacity\":1}" >/dev/null || true
  done
  info "disponibilidade do Centro: seg–sex 09–12 e 14–18"
fi
if [ -n "$LOC_ONLINE" ]; then
  for wd in 2 4; do
    req POST "$ADMIN/locations/$LOC_ONLINE/availability" \
      "{\"weekday\":$wd,\"start_time\":\"19:00\",\"end_time\":\"21:00\",\"capacity\":1}" >/dev/null || true
  done
  info "disponibilidade Online: ter/qui 19–21"
fi

# --- 3) Pacientes aprovados (cadastrados pelo terapeuta) -------------------
info "Pacientes (cadastrados pelo terapeuta → nascem aprovados)"
P_ANA=$(create_id POST "$ADMIN/patients" \
  "{\"name\":\"Ana Souza\",\"phone\":\"11999990001\",\"email\":\"ana.souza@example.com\",\"cpf\":\"39053344705\",\"origin_id\":\"$ORIGIN_INDICACAO\"}" \
  "paciente: Ana Souza")
P_BRUNO=$(create_id POST "$ADMIN/patients" \
  "{\"name\":\"Bruno Lima\",\"phone\":\"11999990002\",\"email\":\"bruno.lima@example.com\",\"cpf\":\"\",\"origin_id\":\"$ORIGIN_DOCTORALIA\"}" \
  "paciente: Bruno Lima")
P_CARLA=$(create_id POST "$ADMIN/patients" \
  "{\"name\":\"Carla Dias\",\"phone\":\"11999990003\",\"email\":\"carla.dias@example.com\",\"cpf\":\"\",\"origin_id\":\"$ORIGIN_INSTAGRAM\"}" \
  "paciente: Carla Dias")

# --- 4) Auto-cadastro pelo portal (nasce PENDENTE) -------------------------
# O cadastro do portal é PUT /register (upsert por email), não POST.
info "Auto-cadastro pelo portal (fica pendente de aprovação)"
if req PUT "$PORTAL/register" \
  '{"credential":"diego.pendente@example.com","name":"Diego Pendente","phone":"11999990004","cpf":""}' >/dev/null; then
  info "portal: Diego Pendente cadastrado (status pendente — aprove em /app → Aprovações)"
fi

# --- 5) Planos (um de cada tipo relevante) ---------------------------------
info "Planos por paciente"
START=$(iso 0 "00:00")
[ -n "$P_ANA" ]   && post "$ADMIN/plans" \
  "{\"patient_id\":\"$P_ANA\",\"type\":\"pagamento_por_consulta\",\"amount\":20000,\"starts_at\":\"$START\"}" \
  "plano de Ana: pagamento por consulta (R\$ 200,00)"
[ -n "$P_BRUNO" ] && post "$ADMIN/plans" \
  "{\"patient_id\":\"$P_BRUNO\",\"type\":\"plano_fechado_mensal\",\"amount\":80000,\"starts_at\":\"$START\"}" \
  "plano de Bruno: fechado mensal (R\$ 800,00)"
[ -n "$P_CARLA" ] && post "$ADMIN/plans" \
  "{\"patient_id\":\"$P_CARLA\",\"type\":\"atendimento_social\",\"amount\":0,\"starts_at\":\"$START\"}" \
  "plano de Carla: atendimento social (sem cobrança)"

# --- 6) Custos (categorias + itens) ----------------------------------------
# kind válido: local|crp|infra|plataforma (internal/domain/cost.go).
info "Custos (categorias e itens — base do ROI)"
CAT_PLAT=$(create_id POST "$ADMIN/costs/categories" '{"kind":"plataforma","name":"Plataformas de aquisição"}' "categoria: Plataformas")
CAT_INFRA=$(create_id POST "$ADMIN/costs/categories" '{"kind":"infra","name":"Infraestrutura"}' "categoria: Infraestrutura")
if [ -n "$CAT_PLAT" ]; then
  post "$ADMIN/costs/items" \
    "{\"category_id\":\"$CAT_PLAT\",\"name\":\"Assinatura Doctoralia\",\"amount\":15000,\"period\":\"mensal\",\"origin_id\":\"$ORIGIN_DOCTORALIA\"}" \
    "item: Assinatura Doctoralia (R\$ 150,00/mês, ligada à origem Doctoralia)"
fi
if [ -n "$CAT_INFRA" ]; then
  post "$ADMIN/costs/items" \
    "{\"category_id\":\"$CAT_INFRA\",\"name\":\"Hospedagem/VM\",\"amount\":6000,\"period\":\"mensal\"}" \
    "item: Hospedagem/VM (R\$ 60,00/mês)"
fi

# --- 7) Sessões (status "solicitada": não aciona o Google) -----------------
info "Sessões de exemplo (status solicitada)"
if [ -n "$P_ANA" ] && [ -n "$LOC_CENTRO" ]; then
  post "$ADMIN/sessions" \
    "{\"patient_id\":\"$P_ANA\",\"location_id\":\"$LOC_CENTRO\",\"modality\":\"presencial\",\"starts_at\":\"$(iso 1 14:00)\",\"ends_at\":\"$(iso 1 15:00)\",\"status\":\"solicitada\"}" \
    "sessão solicitada: Ana — amanhã 14:00 (Centro)"
fi
if [ -n "$P_BRUNO" ] && [ -n "$LOC_ONLINE" ]; then
  post "$ADMIN/sessions" \
    "{\"patient_id\":\"$P_BRUNO\",\"location_id\":\"$LOC_ONLINE\",\"modality\":\"online\",\"starts_at\":\"$(iso 2 19:00)\",\"ends_at\":\"$(iso 2 20:00)\",\"status\":\"solicitada\"}" \
    "sessão solicitada: Bruno — em 2 dias 19:00 (Online)"
fi

# --- resumo ----------------------------------------------------------------
info ""
info "Carga concluída. Para navegar:"
info "   Admin:  ${ADMIN_URL}/app/"
info "   Portal: ${PORTAL_URL}/app/"
info "${c_dim}   Dica: no admin, abra \"Aprovações\" para liberar o paciente pendente (Diego).${c_off}"
