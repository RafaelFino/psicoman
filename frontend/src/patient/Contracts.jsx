import { useEffect, useState } from 'react'
import { patientApi } from '../api'

export default function PatientContracts() {
  const [contracts, setContracts] = useState([])
  const [viewing, setViewing] = useState(null)
  const [error, setError] = useState('')
  const [signing, setSigning] = useState(false)

  const load = () => patientApi.contracts().then(setContracts).catch(e => setError(e.message))
  useEffect(() => { load() }, [])

  const sign = async (id) => {
    if (!confirm('Ao assinar, você declara que leu e concorda com os termos do contrato. Deseja continuar?')) return
    setSigning(true)
    try {
      await patientApi.signContract(id)
      setViewing(null)
      load()
    } catch (err) { setError(err.message) }
    finally { setSigning(false) }
  }

  const statusLabel = (s) => ({
    pending: 'Pendente de assinatura',
    signed: 'Assinado',
    expired: 'Expirado',
    revoked: 'Revogado',
  }[s] || s)

  const statusClass = (s) => ({
    pending: 'scheduled',
    signed: 'completed',
    expired: 'cancelled',
    revoked: 'cancelled',
  }[s] || '')

  if (viewing) {
    return (
      <div>
        <h2>Contrato Terapêutico</h2>
        <button className="btn secondary" onClick={() => setViewing(null)} style={{ marginBottom: '1rem' }}>Voltar</button>

        <div className="card">
          <div dangerouslySetInnerHTML={{ __html: viewing.generated_html }} style={{ lineHeight: 1.7 }} />
        </div>

        {viewing.status === 'pending' && (
          <div className="card" style={{ background: '#f0fdf4', borderColor: '#86efac' }}>
            <h3>Assinatura digital</h3>
            <p>Ao clicar em "Assinar contrato", você declara que:</p>
            <ul>
              <li>Leu e compreendeu todos os termos acima</li>
              <li>Concorda com as condições estabelecidas</li>
              <li>Sua assinatura digital tem validade legal</li>
            </ul>
            <button className="btn" onClick={() => sign(viewing.id)} disabled={signing}>
              {signing ? 'Assinando...' : 'Assinar contrato'}
            </button>
          </div>
        )}

        {viewing.status === 'signed' && (
          <div className="card" style={{ background: '#f0fdf4', borderColor: '#86efac' }}>
            <p style={{ color: 'var(--success)' }}>
              Contrato assinado em {new Date(viewing.signed_at).toLocaleString('pt-BR')}
            </p>
          </div>
        )}
      </div>
    )
  }

  return (
    <div>
      <h2>Meus contratos</h2>
      {error && <p className="error">{error}</p>}
      {(contracts || []).length === 0 ? <p className="muted">Nenhum contrato disponível.</p> : (
        <div className="grid">
          {contracts.map(c => (
            <div key={c.id} className="card">
              <strong>{c.template_name}</strong>
              <p><span className={`badge ${statusClass(c.status)}`}>{statusLabel(c.status)}</span></p>
              <p className="muted">{new Date(c.created_at).toLocaleDateString('pt-BR')}</p>
              <button className="btn" onClick={() => setViewing(c)}>
                {c.status === 'pending' ? 'Ler e assinar' : 'Visualizar'}
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
