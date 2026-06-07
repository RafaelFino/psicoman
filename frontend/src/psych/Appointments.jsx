import { useEffect, useState } from 'react'
import { psychApi } from '../api'

export default function PsychAppointments() {
  const [appts, setAppts] = useState([])
  const [patients, setPatients] = useState([])
  const [selected, setSelected] = useState(null)
  const [notes, setNotes] = useState('')
  const [report, setReport] = useState('')
  const [form, setForm] = useState({ patient_id: '', type: 'online', scheduled_at: '', duration_minutes: 50 })
  const [error, setError] = useState('')

  const load = () => {
    psychApi.appointments().then(setAppts).catch((e) => setError(e.message))
    psychApi.patients().then(setPatients)
  }
  useEffect(() => { load() }, [])

  const create = async (e) => {
    e.preventDefault()
    try {
      await psychApi.createAppointment({ ...form, scheduled_at: new Date(form.scheduled_at).toISOString() })
      load()
    } catch (err) { setError(err.message) }
  }

  const saveNotes = async () => {
    try {
      await psychApi.updateNotes(selected.id, { notes, report_html: report })
      load()
    } catch (err) { setError(err.message) }
  }

  const openEditor = (a) => {
    setSelected(a)
    setNotes(a.notes || '')
    setReport(a.report_html || '')
  }

  return (
    <div>
      <h2>Atendimentos</h2>
      {error && <p className="error">{error}</p>}

      <form className="card grid grid-2" onSubmit={create}>
        <div>
          <label>Paciente</label>
          <select value={form.patient_id} onChange={(e) => setForm({ ...form, patient_id: e.target.value })} required>
            <option value="">Selecione</option>
            {patients.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
          </select>
          <label>Tipo</label>
          <select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
            <option value="online">Online</option>
            <option value="in_person">Presencial</option>
          </select>
        </div>
        <div>
          <label>Data e hora</label>
          <input type="datetime-local" value={form.scheduled_at} onChange={(e) => setForm({ ...form, scheduled_at: e.target.value })} required />
          <button className="btn" type="submit">Agendar</button>
        </div>
      </form>

      <div className="card">
        <table>
          <thead><tr><th>Paciente</th><th>Data</th><th>Status</th><th>Meet</th><th></th></tr></thead>
          <tbody>
            {appts.map((a) => (
              <tr key={a.id}>
                <td>{a.patient_name}</td>
                <td>{new Date(a.scheduled_at).toLocaleString('pt-BR')}</td>
                <td><span className={`badge ${a.status}`}>{a.status}</span></td>
                <td>{a.meet_link ? <a href={a.meet_link} target="_blank" rel="noreferrer">Abrir</a> : '—'}</td>
                <td>
                  <button className="btn secondary" onClick={() => openEditor(a)}>Anotar</button>
                  {a.status === 'scheduled' && (
                    <button className="btn secondary" onClick={() => psychApi.completeAppointment(a.id).then(load)}>Concluir</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {selected && (
        <div className="card">
          <h3>Relatório — {selected.patient_name}</h3>
          <label>Anotações</label>
          <textarea value={notes} onChange={(e) => setNotes(e.target.value)} rows={4} />
          <label>Relatório</label>
          <textarea className="editor" value={report} onChange={(e) => setReport(e.target.value)} rows={8} />
          <button className="btn" onClick={saveNotes}>Salvar</button>
        </div>
      )}
    </div>
  )
}
