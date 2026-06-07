import { useEffect, useState } from 'react'
import { patientApi } from '../api'

export default function PatientDashboard() {
  const [appts, setAppts] = useState([])
  const [error, setError] = useState('')

  const load = () => patientApi.appointments().then(setAppts).catch((e) => setError(e.message))
  useEffect(() => { load() }, [])

  const cancel = async (id) => {
    if (!confirm('Deseja cancelar este atendimento?')) return
    try {
      await patientApi.cancel(id, 'Cancelado pelo paciente')
      load()
    } catch (err) { setError(err.message) }
  }

  return (
    <div>
      <h2>Meus atendimentos</h2>
      {error && <p className="error">{error}</p>}
      {appts.length === 0 ? <p className="muted">Nenhum atendimento agendado.</p> : (
        <div className="grid">
          {appts.map((a) => (
            <div key={a.id} className="card">
              <p><strong>{new Date(a.scheduled_at).toLocaleString('pt-BR')}</strong></p>
              <span className={`badge ${a.status}`}>{a.status}</span>
              <p>{a.type === 'online' ? 'Online' : 'Presencial'}</p>
              {a.meet_link && <p><a href={a.meet_link} target="_blank" rel="noreferrer">Entrar no Google Meet</a></p>}
              {a.status === 'scheduled' && (
                <button className="btn danger" onClick={() => cancel(a.id)}>Cancelar</button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
