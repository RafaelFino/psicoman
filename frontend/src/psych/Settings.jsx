import { useEffect, useState } from 'react'
import { psychApi } from '../api'

export default function PsychSettings() {
  const [rules, setRules] = useState(null)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    psychApi.rules().then(setRules).catch((e) => setError(e.message))
  }, [])

  const save = async () => {
    try {
      await psychApi.updateRules(rules)
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch (err) { setError(err.message) }
  }

  if (!rules) return <p>Carregando...</p>

  return (
    <div>
      <h2>Configurações</h2>
      {error && <p className="error">{error}</p>}
      {saved && <p style={{ color: 'var(--success)' }}>Salvo!</p>}

      <div className="card">
        <h3>Regras de agendamento</h3>
        <label>Horas mínimas para cancelar (paciente)</label>
        <input type="number" value={rules.min_hours_to_cancel} onChange={(e) => setRules({ ...rules, min_hours_to_cancel: +e.target.value })} />
        <label>Horas mínimas para reagendar (paciente)</label>
        <input type="number" value={rules.min_hours_to_reschedule} onChange={(e) => setRules({ ...rules, min_hours_to_reschedule: +e.target.value })} />
        <label>Máximo de reagendamentos por mês</label>
        <input type="number" value={rules.max_reschedules_per_month} onChange={(e) => setRules({ ...rules, max_reschedules_per_month: +e.target.value })} />
        <label><input type="checkbox" checked={rules.allow_patient_cancel} onChange={(e) => setRules({ ...rules, allow_patient_cancel: e.target.checked })} /> Paciente pode cancelar</label>
        <br />
        <label><input type="checkbox" checked={rules.allow_patient_reschedule} onChange={(e) => setRules({ ...rules, allow_patient_reschedule: e.target.checked })} /> Paciente pode reagendar</label>
        <br /><br />
        <button className="btn" onClick={save}>Salvar regras</button>
      </div>

      <div className="card">
        <h3>Google Calendar</h3>
        <p className="muted">Conecte sua conta Google para sincronizar agenda e criar links do Meet.</p>
        <button className="btn" onClick={async () => {
          const res = await fetch('/api/psych/google/auth')
          const data = await res.json()
          if (data.url) window.location.href = data.url
        }}>Conectar Google</button>
      </div>
    </div>
  )
}
