#!/bin/bash
# Seed script para dados de teste — cria pacientes com prefixo "TEST" e emails @test.com.
# Uso: bash scripts/seed-test-data.sh
# Requer: servidor rodando em localhost:8080 com DEV_MODE=true
#
# Estes dados podem ser removidos via: DELETE /api/dev/test-data

set -u

BASE_URL="${BASE_URL:-http://localhost:8080}"
AUTH_HEADER="X-Dev-Auth: dev-local"
CONTENT_TYPE="Content-Type: application/json"
PATIENT_COUNT=0
APPT_COUNT=0

# ─── Helper ─────────────────────────────────────────────────────

api_call() {
    local method=$1 path=$2 body=$3
    local response status
    response=$(curl -s -w "\n%{http_code}" -X "$method" \
        -H "$AUTH_HEADER" -H "$CONTENT_TYPE" \
        -d "$body" "$BASE_URL$path")
    status=$(echo "$response" | tail -1)
    body_content=$(echo "$response" | sed '$d')
    if [ "$status" -ge 200 ] && [ "$status" -lt 300 ]; then
        echo "$body_content"
        return 0
    else
        echo "  ⚠ ERRO ($status): $body_content" >&2
        return 1
    fi
}

# ─── Date arithmetic ────────────────────────────────────────────

CURRENT_YEAR=$(date -u +%Y)
CURRENT_MONTH=$(date -u +%m)
# Previous month
PREV_MONTH=$((10#$CURRENT_MONTH - 1))
PREV_YEAR=$CURRENT_YEAR
if [ $PREV_MONTH -eq 0 ]; then PREV_MONTH=12; PREV_YEAR=$((PREV_YEAR - 1)); fi

# Format months with leading zero
CURRENT_MONTH_FMT=$(printf "%02d" $((10#$CURRENT_MONTH)))
PREV_MONTH_FMT=$(printf "%02d" $PREV_MONTH)

# ─── Create patients ────────────────────────────────────────────

echo "═══════════════════════════════════════"
echo "  Criando pacientes de teste..."
echo "═══════════════════════════════════════"
echo ""

PATIENT_IDS=()

create_patient() {
    local name=$1 email=$2 phone=$3
    local resp
    resp=$(api_call POST "/api/psych/patients" \
        "{\"name\":\"$name\",\"email\":\"$email\",\"phone\":\"$phone\"}")
    if [ $? -eq 0 ]; then
        local pid
        pid=$(echo "$resp" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [ -n "$pid" ]; then
            echo "  ✓ $name ($pid)"
            PATIENT_IDS+=("$pid")
            PATIENT_COUNT=$((PATIENT_COUNT + 1))
        else
            echo "  ⚠ $name: resposta sem ID — $resp" >&2
        fi
    else
        echo "  ⚠ Falha ao criar $name" >&2
    fi
}

create_patient "TEST Ana Silva" "test.ana@test.com" "(11) 91111-0001"
create_patient "TEST Bruno Costa" "test.bruno@test.com" "(11) 91111-0002"
create_patient "TEST Carla Lima" "test.carla@test.com" "(11) 91111-0003"

echo ""

# ─── Create appointments ────────────────────────────────────────

echo "═══════════════════════════════════════"
echo "  Criando consultas de teste..."
echo "═══════════════════════════════════════"
echo ""

create_appointment() {
    local patient_id=$1 scheduled_at=$2 appt_type=$3
    local resp
    resp=$(api_call POST "/api/psych/appointments" \
        "{\"patient_id\":\"$patient_id\",\"type\":\"$appt_type\",\"scheduled_at\":\"$scheduled_at\",\"duration_minutes\":50}")
    if [ $? -eq 0 ]; then
        echo "  ✓ $scheduled_at ($appt_type)"
        APPT_COUNT=$((APPT_COUNT + 1))
    else
        echo "  ⚠ Falha: $scheduled_at ($appt_type)" >&2
    fi
}

for pid in "${PATIENT_IDS[@]}"; do
    patient_name=$(echo "$pid" | cut -c1-8)
    echo "  Paciente $patient_name..."

    # Current month — day 5 at 09:00 (online)
    create_appointment "$pid" "${CURRENT_YEAR}-${CURRENT_MONTH_FMT}-05T09:00:00Z" "online"

    # Current month — day 15 at 14:00 (in_person)
    create_appointment "$pid" "${CURRENT_YEAR}-${CURRENT_MONTH_FMT}-15T14:00:00Z" "in_person"

    # Previous month — day 8 at 09:00 (online)
    create_appointment "$pid" "${PREV_YEAR}-${PREV_MONTH_FMT}-08T09:00:00Z" "online"

    # Previous month — day 20 at 14:00 (in_person)
    create_appointment "$pid" "${PREV_YEAR}-${PREV_MONTH_FMT}-20T14:00:00Z" "in_person"

    echo ""
done

# ─── Summary ────────────────────────────────────────────────────

echo "═══════════════════════════════════════"
echo "  Seed concluído!"
echo "  Pacientes criados: $PATIENT_COUNT"
echo "  Consultas criadas: $APPT_COUNT"
echo "═══════════════════════════════════════"
