#!/bin/bash
# Seed script — popula o sistema com 10 pacientes e consultas nos dias úteis.
# Uso: bash scripts/seed.sh
# Requer: servidor rodando em localhost:8080 com DEV_MODE=true

BASE="http://localhost:8080"
SECRET="dev-local"
AUTH="-H X-Dev-Auth: $SECRET"

echo "=== Criando 10 pacientes ==="

PATIENTS=()
NAMES=(
  "Ana Beatriz Silva:ana.silva@email.com:11999001001"
  "Bruno Costa Souza:bruno.costa@email.com:11999002002"
  "Carla Mendes Oliveira:carla.mendes@email.com:11999003003"
  "Daniel Rocha Lima:daniel.rocha@email.com:11999004004"
  "Elisa Ferreira Santos:elisa.ferreira@email.com:11999005005"
  "Fernando Alves Pereira:fernando.alves@email.com:11999006006"
  "Gabriela Nunes Martins:gabriela.nunes@email.com:11999007007"
  "Hugo Teixeira Barbosa:hugo.teixeira@email.com:11999008008"
  "Isabela Ramos Correia:isabela.ramos@email.com:11999009009"
  "João Pedro Cardoso:joao.pedro@email.com:11999010010"
)

for entry in "${NAMES[@]}"; do
  IFS=':' read -r name email phone <<< "$entry"
  resp=$(curl -s -X POST "$BASE/api/psych/patients" \
    -H "X-Dev-Auth: $SECRET" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"$name\",\"email\":\"$email\",\"phone\":\"$phone\"}")
  
  # Extrair ID do paciente
  pid=$(echo "$resp" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  
  if [ -z "$pid" ]; then
    echo "  ERRO criando $name: $resp"
  else
    echo "  ✓ $name ($pid)"
    PATIENTS+=("$pid")
  fi
done

echo ""
echo "=== Criando consultas (3/dia, dias úteis desta semana e próxima) ==="

# Gerar dias úteis: semana atual + próxima semana (seg a sex)
DAYS=()
# Achar próxima segunda (ou hoje se já é segunda)
today=$(date +%u)  # 1=seg, 7=dom
if [ "$today" -le 5 ]; then
  # Estamos num dia útil, começar da segunda desta semana
  offset=$((1 - today))
else
  # Fim de semana, começar da próxima segunda
  offset=$((8 - today))
fi

for i in $(seq 0 9); do
  d=$(date -d "+$((offset + i)) days" +%Y-%m-%d 2>/dev/null || date -v+$((offset + i))d +%Y-%m-%d)
  dow=$(date -d "$d" +%u 2>/dev/null || date -j -f "%Y-%m-%d" "$d" +%u)
  if [ "$dow" -le 5 ]; then
    DAYS+=("$d")
  fi
done

# Horários das consultas (3 por dia, intercalando horários)
HOURS=("09:00" "10:00" "11:00" "14:00" "15:00" "16:00")
TYPES=("online" "in_person" "online" "in_person" "online" "in_person")

patient_idx=0
num_patients=${#PATIENTS[@]}
appt_count=0

for day in "${DAYS[@]}"; do
  # 3 consultas por dia, usando horários diferentes
  slot_start=$(( (appt_count * 3) % 6 ))
  for s in 0 1 2; do
    slot_idx=$(( (slot_start + s) % 6 ))
    hour="${HOURS[$slot_idx]}"
    type="${TYPES[$slot_idx]}"
    pid="${PATIENTS[$patient_idx]}"
    
    scheduled="${day}T${hour}:00Z"
    
    resp=$(curl -s -X POST "$BASE/api/psych/appointments" \
      -H "X-Dev-Auth: $SECRET" \
      -H "Content-Type: application/json" \
      -d "{\"patient_id\":\"$pid\",\"type\":\"$type\",\"scheduled_at\":\"$scheduled\",\"duration_minutes\":50}")
    
    status=$(echo "$resp" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
    pname=$(echo "$resp" | grep -o '"patient_name":"[^"]*"' | cut -d'"' -f4)
    
    if [ "$status" = "scheduled" ]; then
      echo "  ✓ $day $hour — $pname ($type)"
      appt_count=$((appt_count + 1))
    else
      echo "  ✗ $day $hour — $resp"
    fi
    
    # Rotacionar pacientes
    patient_idx=$(( (patient_idx + 1) % num_patients ))
  done
done

echo ""
echo "=== Adicionando pagamentos pendentes ==="

for i in $(seq 0 $((num_patients - 1))); do
  pid="${PATIENTS[$i]}"
  amount=$(( (RANDOM % 20 + 15) * 1000 ))  # R$150-350
  due="${DAYS[0]}T00:00:00Z"
  
  resp=$(curl -s -X POST "$BASE/api/psych/finance/payments" \
    -H "X-Dev-Auth: $SECRET" \
    -H "Content-Type: application/json" \
    -d "{\"patient_id\":\"$pid\",\"amount_cents\":$amount,\"status\":\"pending\",\"due_date\":\"$due\"}")
  
  echo "  ✓ Pagamento R\$$(echo "scale=2; $amount/100" | bc) — paciente $((i+1))"
done

echo ""
echo "=== Adicionando custos mensais ==="

month=$(date +%-m)
year=$(date +%Y)

COSTS=(
  "Aluguel consultório:250000:espaco"
  "Internet:15000:infraestrutura"
  "Supervisor Dr. Carlos:40000:supervisao"
  "Material de escritório:8500:material"
)

for entry in "${COSTS[@]}"; do
  IFS=':' read -r desc amount cat <<< "$entry"
  curl -s -X POST "$BASE/api/psych/finance/costs" \
    -H "X-Dev-Auth: $SECRET" \
    -H "Content-Type: application/json" \
    -d "{\"description\":\"$desc\",\"amount_cents\":$amount,\"month\":$month,\"year\":$year,\"category\":\"$cat\"}" > /dev/null
  echo "  ✓ $desc — R\$$(echo "scale=2; $amount/100" | bc)"
done

echo ""
echo "=== Seed completo! ==="
echo "Pacientes: $num_patients"
echo "Consultas: $appt_count"
echo "Dias úteis: ${#DAYS[@]} (${DAYS[0]} a ${DAYS[-1]})"
echo ""
echo "Acesse:"
echo "  Psicólogo: http://localhost:5173/psych"
echo "  Paciente:  http://localhost:5173/patient/login (aba Dev)"
