import { useEffect, useState } from 'react'
import { patientApi } from '../api'

export default function PatientAnamnesis() {
  const [text, setText] = useState('')
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    patientApi.me().then((p) => setText(p.anamnesis || '')).catch((e) => setError(e.message))
  }, [])

  const save = async () => {
    try {
      await patientApi.anamnesis({ anamnesis: text })
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch (err) { setError(err.message) }
  }

  return (
    <div>
      <h2>Anamnese</h2>
      <p className="muted">Preencha suas informações para ajudar no atendimento.</p>
      {error && <p className="error">{error}</p>}
      {saved && <p style={{ color: 'var(--success)' }}>Salvo!</p>}
      <div className="card">
        <textarea value={text} onChange={(e) => setText(e.target.value)} rows={12} placeholder="Descreva seu histórico, queixas principais, medicamentos em uso..." />
        <button className="btn" onClick={save}>Salvar anamnese</button>
      </div>
    </div>
  )
}
