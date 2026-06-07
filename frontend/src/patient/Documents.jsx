import { useEffect, useState } from 'react'
import { patientApi } from '../api'

async function downloadDoc(id, filename) {
  const token = localStorage.getItem('patient_token')
  const res = await fetch(`/api/patient/documents/${id}/download`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!res.ok) return
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

export default function PatientDocuments() {
  const [docs, setDocs] = useState([])
  const [error, setError] = useState('')

  useEffect(() => {
    patientApi.documents().then(setDocs).catch((e) => setError(e.message))
  }, [])

  return (
    <div>
      <h2>Meus documentos</h2>
      {error && <p className="error">{error}</p>}
      {docs.length === 0 ? <p className="muted">Nenhum documento disponível.</p> : (
        <div className="card">
          <table>
            <thead><tr><th>Arquivo</th><th>Tipo</th><th>Data</th><th></th></tr></thead>
            <tbody>
              {docs.map((d) => (
                <tr key={d.id}>
                  <td>{d.filename}</td>
                  <td>{d.doc_type}</td>
                  <td>{new Date(d.created_at).toLocaleDateString('pt-BR')}</td>
                  <td><button className="btn secondary" onClick={() => downloadDoc(d.id, d.filename)}>Baixar</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
