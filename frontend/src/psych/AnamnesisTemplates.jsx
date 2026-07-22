import { useEffect, useState } from 'react'
import { psychApi } from '../api'

const FIELD_TYPES = [
  { value: 'text', label: 'Texto curto' },
  { value: 'textarea', label: 'Texto longo' },
  { value: 'select', label: 'Seleção' },
  { value: 'checkbox', label: 'Checkbox' },
  { value: 'date', label: 'Data' },
  { value: 'number', label: 'Número' },
  { value: 'scale', label: 'Escala (1-10)' },
]

const AGE_GROUPS = [
  { value: 'adult', label: 'Adulto' },
  { value: 'child', label: 'Criança' },
  { value: 'universal', label: 'Universal' },
]

function emptyField() {
  return { key: '', label: '', type: 'text', required: false, options: [] }
}

export default function PsychAnamnesisTemplates() {
  const [templates, setTemplates] = useState([])
  const [responses, setResponses] = useState([])
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState({ name: '', target_age_group: 'adult', fields: [emptyField()] })
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  const [tab, setTab] = useState('templates')
  const [viewResponse, setViewResponse] = useState(null)

  const load = () => {
    psychApi.anamnesisTemplates().then(setTemplates).catch(e => setError(e.message))
    psychApi.anamnesisResponses('').then(setResponses).catch(() => {})
  }
  useEffect(() => { load() }, [])

  const addField = () => setForm({ ...form, fields: [...form.fields, emptyField()] })
  const removeField = (idx) => setForm({ ...form, fields: form.fields.filter((_, i) => i !== idx) })
  const updateField = (idx, key, value) => {
    const fields = [...form.fields]
    fields[idx] = { ...fields[idx], [key]: value }
    // Auto-generate key from label
    if (key === 'label' && !editing) {
      fields[idx].key = value.toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/(^_|_$)/g, '')
    }
    setForm({ ...form, fields })
  }

  const submit = async (e) => {
    e.preventDefault()
    setError('')
    try {
      const data = { ...form, fields: form.fields.filter(f => f.label) }
      if (editing) {
        await psychApi.updateAnamnesisTemplate(editing, { ...data, is_active: true })
      } else {
        await psychApi.createAnamnesisTemplate(data)
      }
      setForm({ name: '', target_age_group: 'adult', fields: [emptyField()] })
      setEditing(null)
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
      load()
    } catch (err) { setError(err.message) }
  }

  const edit = (t) => {
    setEditing(t.id)
    setForm({ name: t.name, target_age_group: t.target_age_group, fields: t.fields?.length ? t.fields : [emptyField()] })
    setTab('templates')
  }

  const remove = async (id) => {
    if (!confirm('Excluir este template?')) return
    try {
      await psychApi.deleteAnamnesisTemplate(id)
      load()
    } catch (err) { setError(err.message) }
  }

  return (
    <div>
      <h2>Anamnese</h2>
      {error && <p className="error">{error}</p>}
      {saved && <p style={{ color: 'var(--success)' }}>Salvo!</p>}

      <div className="nav" style={{ marginBottom: '1rem' }}>
        <button className={`btn ${tab === 'templates' ? '' : 'secondary'}`} onClick={() => setTab('templates')}>Templates</button>
        <button className={`btn ${tab === 'responses' ? '' : 'secondary'}`} onClick={() => setTab('responses')}>Respostas</button>
      </div>

      {tab === 'templates' && (
        <>
          <div className="card">
            <h3>{editing ? 'Editar template' : 'Novo template'}</h3>
            <form onSubmit={submit}>
              <div className="grid grid-2">
                <div>
                  <label>Nome do template</label>
                  <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} placeholder="Ex: Anamnese Adulto Padrão" required />
                </div>
                <div>
                  <label>Faixa etária</label>
                  <select value={form.target_age_group} onChange={e => setForm({ ...form, target_age_group: e.target.value })}>
                    {AGE_GROUPS.map(g => <option key={g.value} value={g.value}>{g.label}</option>)}
                  </select>
                </div>
              </div>

              <h4>Campos do formulário</h4>
              {form.fields.map((field, idx) => (
                <div key={idx} style={{ border: '1px solid var(--border)', borderRadius: 8, padding: '0.75rem', marginBottom: '0.5rem' }}>
                  <div className="grid grid-2" style={{ gap: '0.5rem' }}>
                    <div>
                      <label>Label</label>
                      <input value={field.label} onChange={e => updateField(idx, 'label', e.target.value)} placeholder="Nome do campo" />
                    </div>
                    <div>
                      <label>Tipo</label>
                      <select value={field.type} onChange={e => updateField(idx, 'type', e.target.value)}>
                        {FIELD_TYPES.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
                      </select>
                    </div>
                  </div>
                  {field.type === 'select' && (
                    <div>
                      <label>Opções (separadas por vírgula)</label>
                      <input value={(field.options || []).join(', ')} onChange={e => updateField(idx, 'options', e.target.value.split(',').map(s => s.trim()).filter(Boolean))} placeholder="Opção 1, Opção 2, Opção 3" />
                    </div>
                  )}
                  <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', marginTop: '0.5rem' }}>
                    <label style={{ margin: 0 }}>
                      <input type="checkbox" checked={field.required} onChange={e => updateField(idx, 'required', e.target.checked)} /> Obrigatório
                    </label>
                    <button type="button" className="btn danger" style={{ padding: '0.3rem 0.6rem', fontSize: '0.8rem' }} onClick={() => removeField(idx)}>Remover</button>
                  </div>
                </div>
              ))}
              <button type="button" className="btn secondary" onClick={addField} style={{ marginBottom: '1rem' }}>+ Adicionar campo</button>
              <br />
              <button className="btn" type="submit">{editing ? 'Atualizar' : 'Criar template'}</button>
              {editing && <button className="btn secondary" type="button" style={{ marginLeft: '0.5rem' }} onClick={() => { setEditing(null); setForm({ name: '', target_age_group: 'adult', fields: [emptyField()] }) }}>Cancelar</button>}
            </form>
          </div>

          <div className="card">
            <h3>Templates existentes</h3>
            {(templates || []).length === 0 ? <p className="muted">Nenhum template criado.</p> : (
              <table>
                <thead><tr><th>Nome</th><th>Faixa</th><th>Campos</th><th></th></tr></thead>
                <tbody>
                  {templates.map(t => (
                    <tr key={t.id}>
                      <td>{t.name}</td>
                      <td>{AGE_GROUPS.find(g => g.value === t.target_age_group)?.label || t.target_age_group}</td>
                      <td>{t.fields?.length || 0}</td>
                      <td>
                        <button className="btn secondary" onClick={() => edit(t)}>Editar</button>
                        <button className="btn danger" style={{ marginLeft: '0.25rem' }} onClick={() => remove(t.id)}>Excluir</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </>
      )}

      {tab === 'responses' && (
        <div className="card">
          <h3>Respostas de pacientes</h3>
          {viewResponse ? (
            <div>
              <button className="btn secondary" onClick={() => setViewResponse(null)} style={{ marginBottom: '1rem' }}>Voltar</button>
              <h4>{viewResponse.patient_name} — {viewResponse.template_name}</h4>
              <p className="muted">Respondido em {viewResponse.completed_at ? new Date(viewResponse.completed_at).toLocaleString('pt-BR') : '—'}</p>
              <table>
                <tbody>
                  {Object.entries(viewResponse.responses || {}).map(([key, val]) => (
                    <tr key={key}><td><strong>{key}</strong></td><td>{val}</td></tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            (responses || []).length === 0 ? <p className="muted">Nenhuma resposta recebida.</p> : (
              <table>
                <thead><tr><th>Paciente</th><th>Template</th><th>Data</th><th></th></tr></thead>
                <tbody>
                  {responses.map(r => (
                    <tr key={r.id}>
                      <td>{r.patient_name}</td>
                      <td>{r.template_name}</td>
                      <td>{r.completed_at ? new Date(r.completed_at).toLocaleDateString('pt-BR') : '—'}</td>
                      <td><button className="btn secondary" onClick={() => setViewResponse(r)}>Ver</button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )
          )}
        </div>
      )}
    </div>
  )
}
