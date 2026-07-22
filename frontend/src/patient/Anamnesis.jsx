import { useEffect, useState } from 'react'
import { patientApi } from '../api'

function DynamicField({ field, value, onChange }) {
  switch (field.type) {
    case 'textarea':
      return <textarea value={value} onChange={e => onChange(e.target.value)} rows={4} required={field.required} />
    case 'select':
      return (
        <select value={value} onChange={e => onChange(e.target.value)} required={field.required}>
          <option value="">Selecione</option>
          {(field.options || []).map(opt => <option key={opt} value={opt}>{opt}</option>)}
        </select>
      )
    case 'checkbox':
      return (
        <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <input type="checkbox" checked={value === 'true'} onChange={e => onChange(e.target.checked ? 'true' : 'false')} />
          Sim
        </label>
      )
    case 'date':
      return <input type="date" value={value} onChange={e => onChange(e.target.value)} required={field.required} />
    case 'number':
      return <input type="number" value={value} onChange={e => onChange(e.target.value)} required={field.required} />
    case 'scale':
      return (
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <input type="range" min="1" max="10" value={value || 5} onChange={e => onChange(e.target.value)} style={{ flex: 1 }} />
          <span style={{ minWidth: '2rem', textAlign: 'center', fontWeight: 600 }}>{value || 5}</span>
        </div>
      )
    default: // text
      return <input type="text" value={value} onChange={e => onChange(e.target.value)} required={field.required} />
  }
}

export default function PatientAnamnesis() {
  const [templates, setTemplates] = useState([])
  const [responses, setResponses] = useState([])
  const [selectedTemplate, setSelectedTemplate] = useState(null)
  const [formData, setFormData] = useState({})
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  const [loading, setLoading] = useState(true)

  // Also keep the old free-text anamnesis for backward compat
  const [freeText, setFreeText] = useState('')

  useEffect(() => {
    Promise.all([
      patientApi.anamnesisTemplates().catch(() => []),
      patientApi.anamnesisResponses().catch(() => []),
      patientApi.me().catch(() => ({})),
    ]).then(([tmpls, resps, me]) => {
      setTemplates(tmpls || [])
      setResponses(resps || [])
      setFreeText(me.anamnesis || '')
      setLoading(false)
    })
  }, [])

  const selectTemplate = (t) => {
    setSelectedTemplate(t)
    setFormData({})
    setError('')
  }

  const submit = async (e) => {
    e.preventDefault()
    setError('')
    try {
      await patientApi.submitAnamnesis({
        template_id: selectedTemplate.id,
        responses: formData,
      })
      setSaved(true)
      setSelectedTemplate(null)
      setTimeout(() => setSaved(false), 3000)
      // Reload responses
      const resps = await patientApi.anamnesisResponses()
      setResponses(resps || [])
    } catch (err) { setError(err.message) }
  }

  const saveFreeText = async () => {
    try {
      await patientApi.anamnesis({ anamnesis: freeText })
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch (err) { setError(err.message) }
  }

  if (loading) return <p>Carregando...</p>

  return (
    <div>
      <h2>Anamnese</h2>
      <p className="muted">Preencha as informações solicitadas para ajudar no seu atendimento.</p>
      {error && <p className="error">{error}</p>}
      {saved && <p style={{ color: 'var(--success)' }}>Enviado com sucesso!</p>}

      {/* Structured anamnesis templates */}
      {(templates || []).length > 0 && !selectedTemplate && (
        <div className="card">
          <h3>Formulários disponíveis</h3>
          <div className="grid">
            {templates.map(t => (
              <div key={t.id} className="card" style={{ cursor: 'pointer' }} onClick={() => selectTemplate(t)}>
                <strong>{t.name}</strong>
                <p className="muted">{t.fields?.length || 0} campos</p>
                <button className="btn">Preencher</button>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Fill structured form */}
      {selectedTemplate && (
        <div className="card">
          <h3>{selectedTemplate.name}</h3>
          <button className="btn secondary" onClick={() => setSelectedTemplate(null)} style={{ marginBottom: '1rem' }}>Voltar</button>
          <form onSubmit={submit}>
            {(selectedTemplate.fields || []).map(field => (
              <div key={field.key} style={{ marginBottom: '1rem' }}>
                <label>{field.label} {field.required && <span style={{ color: 'var(--danger)' }}>*</span>}</label>
                <DynamicField
                  field={field}
                  value={formData[field.key] || ''}
                  onChange={val => setFormData({ ...formData, [field.key]: val })}
                />
              </div>
            ))}
            <button className="btn" type="submit">Enviar respostas</button>
          </form>
        </div>
      )}

      {/* Previous responses */}
      {(responses || []).length > 0 && (
        <div className="card">
          <h3>Minhas respostas anteriores</h3>
          {responses.map(r => (
            <div key={r.id} style={{ borderBottom: '1px solid var(--border)', padding: '0.75rem 0' }}>
              <strong>{r.template_name}</strong>
              <span className="muted" style={{ marginLeft: '0.5rem' }}>
                {r.completed_at ? new Date(r.completed_at).toLocaleDateString('pt-BR') : ''}
              </span>
            </div>
          ))}
        </div>
      )}

      {/* Free-text anamnesis (legacy) */}
      <div className="card">
        <h3>Anamnese livre</h3>
        <p className="muted">Informações adicionais que você queira compartilhar.</p>
        <textarea value={freeText} onChange={e => setFreeText(e.target.value)} rows={8} placeholder="Descreva seu histórico, queixas principais, medicamentos em uso..." />
        <button className="btn" onClick={saveFreeText}>Salvar</button>
      </div>
    </div>
  )
}
