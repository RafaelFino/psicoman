import { useEffect, useState } from 'react'
import { patientApi } from '../api'

export default function PatientBook() {
  const [slots, setSlots] = useState([])
  const [error, setError] = useState('')
  const [ok, setOk] = useState(false)

  useEffect(() => {
    patientApi.slots().then(setSlots).catch((e) => setError(e.message))
  }, [])

  const book = async (slot) => {
    try {
      await patientApi.book({
        type: 'online',
        scheduled_at: slot.start,
        duration_minutes: slot.duration_minutes,
      })
      setOk(true)
    } catch (err) { setError(err.message) }
  }

  return (
    <div>
      <h2>Agendar consulta</h2>
      {error && <p className="error">{error}</p>}
      {ok && <p style={{ color: 'var(--success)' }}>Consulta agendada com sucesso!</p>}
      {slots.length === 0 ? <p className="muted">Nenhum horário disponível.</p> : (
        <div className="grid grid-2">
          {slots.slice(0, 20).map((s, i) => (
            <div key={i} className="card">
              <p>{new Date(s.start).toLocaleString('pt-BR')}</p>
              <button className="btn" onClick={() => book(s)}>Reservar</button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
