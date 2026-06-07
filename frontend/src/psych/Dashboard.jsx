import { useEffect, useState } from 'react'
import { psychApi } from '../api'

export default function PsychDashboard() {
  const [appts, setAppts] = useState([])
  const [error, setError] = useState('')

  useEffect(() => {
    psychApi.appointments()
      .then(setAppts)
      .catch((e) => setError(e.message))
  }, [])

  const today = new Date().toDateString()
  const todayAppts = appts.filter((a) => new Date(a.scheduled_at).toDateString() === today)

  return (
    <div>
      <h2>Agenda de hoje</h2>
      {error && <p className="error">{error}</p>}
      {todayAppts.length === 0 ? (
        <p className="muted">Nenhum atendimento hoje.</p>
      ) : (
        <div className="grid">
          {todayAppts.map((a) => (
            <div key={a.id} className="card">
              <strong>{a.patient_name}</strong>
              <p>{new Date(a.scheduled_at).toLocaleString('pt-BR')}</p>
              <span className={`badge ${a.status}`}>{a.status}</span>
              {a.meet_link && <p><a href={a.meet_link} target="_blank" rel="noreferrer">Google Meet</a></p>}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
