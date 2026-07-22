import { useEffect, useState } from 'react'
import { psychApi } from '../api'

export default function PsychSessionNotes() {
  const [notes, setNotes] = useState([])
  const [patients, setPatients] = useState([])
  const [filterPatient, setFilterPatient] = useState('')
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState({
    appointment_id: '', content_html: '', private_notes: '',
    duration_patient_min: 50, duration_analysis_min: 0, duration_admin_min: 0,
  })
  const [appointments, setAppointments] = useState([])
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  const load = () => {
    psychApi.sessionNotes(filterPatient).then(setNotes).catch(e => setError(e.message))
    psychApi.patients().then(setPatients)
    psychApi.appointments().then(setAppointments)
  }
  useEffect(() => { load() }, [filterPatient])

  const submit = async (e) => {
    e.preventDefault()
    setError('')
    try {
      if (editing) {
        await psychApi.updateSessionNote(editing, {
          content_html: form.content_html,
          private_notes: form.private_notes,
          duration_patient_min: +form.duration_patient_min,
          duration_analysis_min: +form.duration_analysis_min,
          duration_admin_min: +form.duration_admin_min,
        })
      } else {
        await psychApi.createSessionNote({
          ...form,
          duration_patient_min: +form.duration_patient_min,
          duration_analysis_min: +form.duration_analysis_min,
          duration_admin_min: +form.duration_admin_min,
        })
      }
      setForm({ appointment_id: '', content_html: '', private_notes: '', duration_patient_min: 50, duration_analysis_min: 0, duration_admin_min: 0 })
      setEditing(null)
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
      load()
    } catch (err) { setError(err.message) }
  }

  const edit = (sn) => {
    setEditing(sn.id)
    setForm({
      appointment_id: sn.appointment_id,
      content_html: sn.content_html,
      private_notes: sn.private_notes,
      duration_patient_min: sn.duration_patient_min,
      duration_analysis_min: sn.duration_analysis_min,
      duration_admin_min: sn.duration_admin_min,
    })
  }

  // Filter completed appointments that don't have notes yet
  const availableAppts = (appointments || []).filter(a =>
    (a.status === 'completed' || a.status === 'scheduled') &&
    !(notes || []).some(n => n.appointment_id === a.id)
  )

  return (
    <div>
      <h2>Evoluções de sessão</h2>
      {error && <p className="error">{error}</p>}
      {saved && <p style={{ color: 'var(--success)' }}>Salvo!</p>}

      <div className="card">
        <h3>{editing ? 'Editar evolução' : 'Nova evolução'}</h3>
        <form onSubmit={submit}>
          {!editing && (
            <>
              <label>Atendimento</label>
              <select value={form.appointment_id} onChange={e => setForm({ ...form, appointment_id: e.target.value })} required>
                <option value="">Selecione um atendimento</option>
                {availableAppts.map(a => (
                  <option key={a.id} value={a.id}>
                    {a.patient_name} — {new Date(a.scheduled_at).toLocaleString('pt-BR')}
                  </option>
                ))}
              </select>
            </>
          )}

          <label>Evolução clínica</label>
          <textarea
            value={form.content_html}
            onChange={e => setForm({ ...form, content_html: e.target.value })}
            rows={6}
            placeholder="Descreva a evolução da sessão, observações clínicas, técnicas utilizadas..."
          />

          <label>Notas privadas (não compartilhadas com paciente)</label>
          <textarea
            value={form.private_notes}
            onChange={e => setForm({ ...form, private_notes: e.target.value })}
            rows={3}
            placeholder="Hipóteses, lembretes, anotações pessoais..."
          />

          <div className="grid grid-2" style={{ gap: '0.5rem' }}>
            <div>
              <label>Tempo com paciente (min)</label>
              <input type="number" min="0" value={form.duration_patient_min} onChange={e => setForm({ ...form, duration_patient_min: e.target.value })} />
            </div>
            <div>
              <label>Tempo de análise (min)</label>
              <input type="number" min="0" value={form.duration_analysis_min} onChange={e => setForm({ ...form, duration_analysis_min: e.target.value })} />
            </div>
            <div>
              <label>Tempo administrativo (min)</label>
              <input type="number" min="0" value={form.duration_admin_min} onChange={e => setForm({ ...form, duration_admin_min: e.target.value })} />
            </div>
          </div>

          <div style={{ marginTop: '1rem' }}>
            <button className="btn" type="submit">{editing ? 'Atualizar' : 'Salvar evolução'}</button>
            {editing && (
              <button className="btn secondary" type="button" style={{ marginLeft: '0.5rem' }} onClick={() => { setEditing(null); setForm({ appointment_id: '', content_html: '', private_notes: '', duration_patient_min: 50, duration_analysis_min: 0, duration_admin_min: 0 }) }}>
                Cancelar
              </button>
            )}
          </div>
        </form>
      </div>

      <div className="card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
          <h3 style={{ margin: 0 }}>Histórico</h3>
          <select value={filterPatient} onChange={e => setFilterPatient(e.target.value)} style={{ width: 'auto' }}>
            <option value="">Todos os pacientes</option>
            {(patients || []).map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
          </select>
        </div>

        {(notes || []).length === 0 ? <p className="muted">Nenhuma evolução registrada.</p> : (
          <div>
            {notes.map(sn => (
              <div key={sn.id} style={{ borderBottom: '1px solid var(--border)', padding: '1rem 0' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                  <div>
                    <strong>{sn.patient_name}</strong>
                    <span className="muted" style={{ marginLeft: '0.5rem' }}>
                      {new Date(sn.created_at).toLocaleDateString('pt-BR')}
                    </span>
                  </div>
                  <button className="btn secondary" onClick={() => edit(sn)}>Editar</button>
                </div>
                <p style={{ whiteSpace: 'pre-wrap', margin: '0.5rem 0' }}>{sn.content_html}</p>
                <div className="muted" style={{ fontSize: '0.85rem' }}>
                  Paciente: {sn.duration_patient_min}min | Análise: {sn.duration_analysis_min}min | Admin: {sn.duration_admin_min}min
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
