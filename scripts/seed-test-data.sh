#!/bin/bash
# Seed completo para dados de teste — cobre TODOS os casos de uso.
# Uso: bash scripts/seed-test-data.sh
# Requer: servidor rodando em localhost:8080 com DEV_MODE=true
#
# Remove dados anteriores: curl -X DELETE localhost:8080/api/dev/test-data -H "X-Dev-Auth: dev-local"

set -u

BASE_URL="${BASE_URL:-http://localhost:8080}"
AUTH="X-Dev-Auth: dev-local"
CT="Content-Type: application/json"
COUNTS=""

# ─── Helper ─────────────────────────────────────────────────────

api() {
    local method=$1 path=$2 body=${3:-}
    local response status
    if [ -n "$body" ]; then
        response=$(curl -s -w "\n%{http_code}" -X "$method" -H "$AUTH" -H "$CT" -d "$body" "$BASE_URL$path")
    else
        response=$(curl -s -w "\n%{http_code}" -X "$method" -H "$AUTH" -H "$CT" "$BASE_URL$path")
    fi
    status=$(echo "$response" | tail -1)
    local content
    content=$(echo "$response" | sed '$d')
    if [ "$status" -ge 200 ] && [ "$status" -lt 300 ]; then
        echo "$content"
        return 0
    else
        echo "  ⚠ ERRO ($status): $content" >&2
        return 1
    fi
}

extract_id() {
    echo "$1" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4
}

# ─── Date arithmetic ────────────────────────────────────────────

YEAR=$(date -u +%Y)
MONTH=$(date -u +%m)
PREV_MONTH=$((10#$MONTH - 1))
PREV_YEAR=$YEAR
if [ $PREV_MONTH -eq 0 ]; then PREV_MONTH=12; PREV_YEAR=$((PREV_YEAR - 1)); fi
M=$(printf "%02d" $((10#$MONTH)))
PM=$(printf "%02d" $PREV_MONTH)

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║        Psicoman — Seed de Dados de Teste                  ║"
echo "╚═══════════════════════════════════════════════════════════╝"
echo ""

# ═══════════════════════════════════════════════════════════════
# 1. ESPAÇOS / CONSULTÓRIOS
# ═══════════════════════════════════════════════════════════════

echo "┌─ Espaços e consultórios ─────────────────────────────────"

S1=$(api POST "/api/psych/spaces" '{"name":"TEST Sala 1 — Fixa","address":"Rua das Flores 123, sala 4","type":"fixed","cost_cents_per_use":0,"cost_cents_monthly":250000,"notes":"Sala principal, ar condicionado"}')
S1_ID=$(extract_id "$S1")
echo "  ✓ Sala 1 Fixa ($S1_ID)"

S2=$(api POST "/api/psych/spaces" '{"name":"TEST Sala 2 — Alugada","address":"Av. Paulista 900, conj 12","type":"rented","cost_cents_per_use":8000,"cost_cents_monthly":0,"notes":"Aluguel por sessão, coworking"}')
S2_ID=$(extract_id "$S2")
echo "  ✓ Sala 2 Alugada ($S2_ID)"

S3=$(api POST "/api/psych/spaces" '{"name":"TEST Sala 3 — Temporária","address":"Centro Clínico ABC","type":"temporary","cost_cents_per_use":5000,"cost_cents_monthly":0,"notes":"Disponível quartas e sextas"}')
echo "  ✓ Sala 3 Temporária"
echo ""

# ═══════════════════════════════════════════════════════════════
# 2. SUPERVISORES
# ═══════════════════════════════════════════════════════════════

echo "┌─ Supervisores ──────────────────────────────────────────"

SUP1=$(api POST "/api/psych/supervisors" '{"name":"TEST Dr. Ricardo Mendes","email":"test.ricardo@test.com","specialty":"Psicanálise","crp":"CRP 06/12345","notes":"Supervisão quinzenal, foco em casos de ansiedade"}')
SUP1_ID=$(extract_id "$SUP1")
echo "  ✓ Dr. Ricardo Mendes ($SUP1_ID)"

SUP2=$(api POST "/api/psych/supervisors" '{"name":"TEST Dra. Juliana Ramos","email":"test.juliana@test.com","specialty":"TCC","crp":"CRP 06/67890","notes":"Supervisão mensal, casos infantis"}')
SUP2_ID=$(extract_id "$SUP2")
echo "  ✓ Dra. Juliana Ramos ($SUP2_ID)"
echo ""

# ═══════════════════════════════════════════════════════════════
# 3. SESSÕES DE SUPERVISÃO
# ═══════════════════════════════════════════════════════════════

echo "┌─ Sessões de supervisão ──────────────────────────────────"

api POST "/api/psych/supervision-sessions" "{\"supervisor_id\":\"$SUP1_ID\",\"scheduled_at\":\"${PREV_YEAR}-${PM}-05T18:00:00Z\",\"duration_minutes\":60,\"topics\":\"Caso Ana - resistência à mudança\",\"cost_cents\":25000}" > /dev/null && echo "  ✓ Supervisão Ricardo - mês anterior"
api POST "/api/psych/supervision-sessions" "{\"supervisor_id\":\"$SUP1_ID\",\"scheduled_at\":\"${YEAR}-${M}-12T18:00:00Z\",\"duration_minutes\":60,\"topics\":\"Caso Bruno - transferência\",\"cost_cents\":25000}" > /dev/null && echo "  ✓ Supervisão Ricardo - mês atual"
api POST "/api/psych/supervision-sessions" "{\"supervisor_id\":\"$SUP2_ID\",\"scheduled_at\":\"${PREV_YEAR}-${PM}-15T17:00:00Z\",\"duration_minutes\":90,\"topics\":\"Caso infantil - limites parentais\",\"cost_cents\":30000}" > /dev/null && echo "  ✓ Supervisão Juliana - mês anterior"
api POST "/api/psych/supervision-sessions" "{\"supervisor_id\":\"$SUP2_ID\",\"scheduled_at\":\"${YEAR}-${M}-20T17:00:00Z\",\"duration_minutes\":90,\"topics\":\"Técnicas de dessensibilização\",\"cost_cents\":30000}" > /dev/null && echo "  ✓ Supervisão Juliana - mês atual"
echo ""

# ═══════════════════════════════════════════════════════════════
# 4. TEMPLATE DE ANAMNESE
# ═══════════════════════════════════════════════════════════════

echo "┌─ Templates de anamnese ──────────────────────────────────"

TMPL=$(api POST "/api/psych/anamnesis/templates" '{"name":"TEST Anamnese Adulto","target_age_group":"adult","fields":[{"key":"queixa","label":"Queixa principal","type":"textarea","required":true},{"key":"medicamentos","label":"Medicamentos em uso","type":"text","required":false},{"key":"historico_familiar","label":"Histórico familiar relevante","type":"textarea","required":false},{"key":"nivel_ansiedade","label":"Nível de ansiedade (1-10)","type":"scale","required":true},{"key":"sono","label":"Qualidade do sono","type":"select","required":true,"options":["Boa","Regular","Ruim","Insônia"]}]}')
TMPL_ID=$(extract_id "$TMPL")
echo "  ✓ Template Anamnese Adulto ($TMPL_ID)"
echo ""

# ═══════════════════════════════════════════════════════════════
# 5. TEMPLATE DE CONTRATO
# ═══════════════════════════════════════════════════════════════

echo "┌─ Template de contrato ──────────────────────────────────"

CTMPL=$(api POST "/api/psych/contracts/templates" '{"name":"TEST Contrato Terapêutico Padrão","content_html":"<h2>Contrato de Prestação de Serviços Psicológicos</h2><p>Eu, <strong>{{PATIENT_NAME}}</strong>, portador do email <strong>{{PATIENT_EMAIL}}</strong>, declaro que estou ciente e de acordo com os termos do atendimento psicológico prestado.</p><h3>Condições</h3><ul><li>Sessões de 50 minutos</li><li>Cancelamento com 24h de antecedência</li><li>Sigilo profissional garantido conforme CFP</li></ul><p>Data: {{DATE}}</p>"}')
CTMPL_ID=$(extract_id "$CTMPL")
echo "  ✓ Contrato Padrão ($CTMPL_ID)"
echo ""

# ═══════════════════════════════════════════════════════════════
# 6. PACIENTES + CONSULTAS + EVOLUÇÕES + PAGAMENTOS + CONTRATOS
# ═══════════════════════════════════════════════════════════════

echo "┌─ Pacientes com dados completos ─────────────────────────"
echo ""

create_full_patient() {
    local name=$1 email=$2 phone=$3 idx=$4
    
    # Create patient
    local resp
    resp=$(api POST "/api/psych/patients" "{\"name\":\"$name\",\"email\":\"$email\",\"phone\":\"$phone\"}")
    local pid
    pid=$(extract_id "$resp")
    if [ -z "$pid" ]; then
        echo "  ⚠ Falha ao criar $name" >&2
        return 1
    fi
    echo "  ✓ Paciente: $name ($pid)"

    # Different days per patient to avoid conflicts
    local D1=$(printf "%02d" $((2 + idx * 2)))
    local D2=$(printf "%02d" $((9 + idx * 2)))
    local D3=$(printf "%02d" $((16 + idx * 2)))
    local D4=$(printf "%02d" $((23 + idx * 2)))
    local H1="09" H2="11" H3="14" H4="16"
    
    # Appointments - current month (2 scheduled, future)
    local A1 A2
    A1=$(api POST "/api/psych/appointments" "{\"patient_id\":\"$pid\",\"type\":\"online\",\"scheduled_at\":\"${YEAR}-${M}-${D1}T${H1}:00:00Z\",\"duration_minutes\":50}")
    local A1_ID=$(extract_id "$A1")
    echo "    ✓ Consulta ${M}/${D1} ${H1}:00 (online)"
    
    A2=$(api POST "/api/psych/appointments" "{\"patient_id\":\"$pid\",\"type\":\"in_person\",\"scheduled_at\":\"${YEAR}-${M}-${D2}T${H2}:00:00Z\",\"duration_minutes\":50}")
    local A2_ID=$(extract_id "$A2")
    echo "    ✓ Consulta ${M}/${D2} ${H2}:00 (presencial)"

    # Appointments - previous month (completed)
    local A3 A4
    A3=$(api POST "/api/psych/appointments" "{\"patient_id\":\"$pid\",\"type\":\"online\",\"scheduled_at\":\"${PREV_YEAR}-${PM}-${D3}T${H3}:00:00Z\",\"duration_minutes\":50}")
    local A3_ID=$(extract_id "$A3")
    echo "    ✓ Consulta ${PM}/${D3} ${H3}:00 (online, passada)"
    
    A4=$(api POST "/api/psych/appointments" "{\"patient_id\":\"$pid\",\"type\":\"in_person\",\"scheduled_at\":\"${PREV_YEAR}-${PM}-${D4}T${H4}:00:00Z\",\"duration_minutes\":50}")
    local A4_ID=$(extract_id "$A4")
    echo "    ✓ Consulta ${PM}/${D4} ${H4}:00 (presencial, passada)"
    
    # Complete past appointments
    if [ -n "$A3_ID" ]; then
        api PATCH "/api/psych/appointments/${A3_ID}/complete" "" > /dev/null 2>&1
    fi
    if [ -n "$A4_ID" ]; then
        api PATCH "/api/psych/appointments/${A4_ID}/complete" "" > /dev/null 2>&1
    fi

    # Session notes for completed appointments
    if [ -n "$A3_ID" ]; then
        api POST "/api/psych/session-notes" "{\"appointment_id\":\"$A3_ID\",\"content_html\":\"Paciente relatou melhora significativa nos sintomas de ansiedade. Técnica de respiração diafragmática aplicada com sucesso. Planejamento para próxima sessão: exposição gradual.\",\"private_notes\":\"Observar evolução do quadro. Considerar redução de frequência para quinzenal.\",\"duration_patient_min\":50,\"duration_analysis_min\":30,\"duration_admin_min\":10}" > /dev/null 2>&1
        echo "    ✓ Evolução registrada (sessão ${PM}/${D3})"
    fi
    if [ -n "$A4_ID" ]; then
        api POST "/api/psych/session-notes" "{\"appointment_id\":\"$A4_ID\",\"content_html\":\"Sessão focada em reestruturação cognitiva. Paciente demonstrou boa capacidade de identificar pensamentos automáticos. Homework: diário de pensamentos.\",\"private_notes\":\"Progresso consistente. Manter abordagem atual.\",\"duration_patient_min\":50,\"duration_analysis_min\":20,\"duration_admin_min\":5}" > /dev/null 2>&1
        echo "    ✓ Evolução registrada (sessão ${PM}/${D4})"
    fi

    # Payments
    api POST "/api/psych/finance/payments" "{\"patient_id\":\"$pid\",\"amount_cents\":25000,\"status\":\"received\",\"due_date\":\"${PREV_YEAR}-${PM}-${D3}T00:00:00Z\"}" > /dev/null 2>&1
    api POST "/api/psych/finance/payments" "{\"patient_id\":\"$pid\",\"amount_cents\":25000,\"status\":\"received\",\"due_date\":\"${PREV_YEAR}-${PM}-${D4}T00:00:00Z\"}" > /dev/null 2>&1
    api POST "/api/psych/finance/payments" "{\"patient_id\":\"$pid\",\"amount_cents\":25000,\"status\":\"pending\",\"due_date\":\"${YEAR}-${M}-${D1}T00:00:00Z\"}" > /dev/null 2>&1
    api POST "/api/psych/finance/payments" "{\"patient_id\":\"$pid\",\"amount_cents\":25000,\"status\":\"pending\",\"due_date\":\"${YEAR}-${M}-${D2}T00:00:00Z\"}" > /dev/null 2>&1
    echo "    ✓ 4 pagamentos (2 recebidos, 2 pendentes)"

    # Contract
    if [ -n "$CTMPL_ID" ]; then
        api POST "/api/psych/contracts" "{\"patient_id\":\"$pid\",\"template_id\":\"$CTMPL_ID\"}" > /dev/null 2>&1
        echo "    ✓ Contrato enviado"
    fi

    echo ""
}

create_full_patient "TEST Ana Silva" "test.ana@test.com" "(11) 91111-0001" 0
create_full_patient "TEST Bruno Costa" "test.bruno@test.com" "(11) 91111-0002" 1
create_full_patient "TEST Carla Lima" "test.carla@test.com" "(11) 91111-0003" 2

# ═══════════════════════════════════════════════════════════════
# 7. CUSTOS OPERACIONAIS
# ═══════════════════════════════════════════════════════════════

echo "┌─ Custos operacionais ────────────────────────────────────"

api POST "/api/psych/finance/costs" "{\"description\":\"TEST Aluguel sala principal\",\"amount_cents\":250000,\"month\":$((10#$MONTH)),\"year\":$YEAR,\"category\":\"aluguel\"}" > /dev/null 2>&1
echo "  ✓ Aluguel sala (R$ 2.500)"

api POST "/api/psych/finance/costs" "{\"description\":\"TEST Internet/telefone\",\"amount_cents\":15000,\"month\":$((10#$MONTH)),\"year\":$YEAR,\"category\":\"telecom\"}" > /dev/null 2>&1
echo "  ✓ Internet/telefone (R$ 150)"

api POST "/api/psych/finance/costs" "{\"description\":\"TEST Supervisão Dr. Ricardo\",\"amount_cents\":25000,\"month\":$((10#$MONTH)),\"year\":$YEAR,\"category\":\"supervisao\"}" > /dev/null 2>&1
echo "  ✓ Supervisão (R$ 250)"

api POST "/api/psych/finance/costs" "{\"description\":\"TEST Material de escritório\",\"amount_cents\":8000,\"month\":$((10#$MONTH)),\"year\":$YEAR,\"category\":\"material\"}" > /dev/null 2>&1
echo "  ✓ Material escritório (R$ 80)"

echo ""

# ═══════════════════════════════════════════════════════════════
# RESUMO
# ═══════════════════════════════════════════════════════════════

echo "╔═══════════════════════════════════════════════════════════╗"
echo "║  Seed completo!                                           ║"
echo "║                                                           ║"
echo "║  • 3 pacientes com histórico completo                     ║"
echo "║  • 12 consultas (4 por paciente, 2 passadas + 2 futuras)  ║"
echo "║  • 6 evoluções de sessão com métricas de tempo            ║"
echo "║  • 12 pagamentos (6 recebidos + 6 pendentes)              ║"
echo "║  • 3 contratos enviados                                   ║"
echo "║  • 3 espaços/consultórios                                 ║"
echo "║  • 2 supervisores + 4 sessões de supervisão               ║"
echo "║  • 1 template de anamnese + 1 template de contrato        ║"
echo "║  • 4 custos operacionais do mês                           ║"
echo "║                                                           ║"
echo "╚═══════════════════════════════════════════════════════════╝"
echo "Cleanup: "
echo " curl -X DELETE localhost:8080/api/dev/test-data "
echo "           -H \"X-Dev-Auth: dev-local\""
