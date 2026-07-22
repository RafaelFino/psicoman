import { useEffect, useState } from 'react'
import { psychApi } from '../api'

export default function PsychContracts() {
  const [templates, setTemplates] = useState([])
  const [contracts, setContracts] = useState([])
  const [patients, setPatients] = useState([])
  const [tab, setTab] = useState('contracts')
  const [templateForm, setTemplateForm] = useState({ name: '', content_html: '' })
  const [contractForm, setContractForm] = useState({ patient_id: '', template_id: '' })
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  const load = () => {
    psychApi.contractTemplates().then(setTemplates).catch(e => setError(e.message))
    psychApi.contracts('').then(setContracts).catch(() => {})
    psychApi.patients().then(setPatients).catch(() => {})
  }
  useEffect(() => { load() }, [])

  const createTemplate = async (e) => {
    e.preventDefault()
    try {
      await psychApi.createContractTemplate(templateForm)
      setTemplateForm({ name: '', content_html: '' })
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
      load()
    } catch (err) { setError(err.message) }
  }

  const sendContract = async (e) => {
    e.preventDefault()
    try {
      await psychApi.createContract(contractForm)
      setContractForm({ patient_id: '', template_id: '' })
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
      load()
    } catch (err) { setError(err.message) }
  }

  const revoke = async (id) => {
    if (!confirm('Revogar este contrato?')) return
    try {
      await psychApi.revokeContract(id)
      load()
    } catch (err) { setError(err.message) }
  }

  const statusLabel = (s) => ({
    pending: 'Pendente',
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

  return (
    <div>
      <h2>Contratos Terapêuticos</h2>
      {error && <p className="error">{error}</p>}
      {saved && <p style={{ color: 'var(--success)' }}>Salvo!</p>}

      <div className="nav" style={{ marginBottom: '1rem' }}>
        <button className={`btn ${tab === 'contracts' ? '' : 'secondary'}`} onClick={() => setTab('contracts')}>Contratos</button>
        <button className={`btn ${tab === 'templates' ? '' : 'secondary'}`} onClick={() => setTab('templates')}>Templates</button>
      </div>

      {tab === 'contracts' && (
        <>
          <div className="card">
            <h3>Enviar contrato para paciente</h3>
            <form onSubmit={sendContract} className="grid grid-2">
              <div>
                <label>Paciente</label>
                <select value={contractForm.patient_id} onChange={e => setContractForm({ ...contractForm, patient_id: e.target.value })} required>
                  <option value="">Selecione</option>
                  {(patients || []).map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
                </select>
              </div>
              <div>
                <label>Template</label>
                <select value={contractForm.template_id} onChange={e => setContractForm({ ...contractForm, template_id: e.target.value })} required>
                  <option value="">Selecione</option>
                  {(templates || []).map(t => <option key={t.id} value={t.id}>{t.name}</option>)}
                </select>
              </div>
              <div><button className="btn" type="submit">Enviar contrato</button></div>
            </form>
          </div>

          <div className="card">
            <h3>Contratos enviados</h3>
            {(contracts || []).length === 0 ? <p className="muted">Nenhum contrato enviado.</p> : (
              <table>
                <thead><tr><th>Paciente</th><th>Template</th><th>Status</th><th>Data</th><th></th></tr></thead>
                <tbody>
                  {contracts.map(c => (
                    <tr key={c.id}>
                      <td>{c.patient_name}</td>
                      <td>{c.template_name}</td>
                      <td><span className={`badge ${statusClass(c.status)}`}>{statusLabel(c.status)}</span></td>
                      <td>{c.signed_at ? new Date(c.signed_at).toLocaleDateString('pt-BR') : new Date(c.created_at).toLocaleDateString('pt-BR')}</td>
                      <td>
                        {c.status === 'pending' && (
                          <button className="btn danger" onClick={() => revoke(c.id)}>Revogar</button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </>
      )}

      {tab === 'templates' && (
        <>
          <div className="card">
            <h3>Novo template de contrato</h3>
            <p className="muted">Use placeholders: {'{{PATIENT_NAME}}'}, {'{{PATIENT_EMAIL}}'}, {'{{PATIENT_PHONE}}'}, {'{{DATE}}'}</p>
            <form onSubmit={createTemplate}>
              <label>Nome</label>
              <input value={templateForm.name} onChange={e => setTemplateForm({ ...templateForm, name: e.target.value })} placeholder="Ex: Contrato Terapêutico Padrão" required />
              <label>Conteúdo HTML</label>
              <textarea
                className="editor"
                value={templateForm.content_html}
                onChange={e => setTemplateForm({ ...templateForm, content_html: e.target.value })}
                rows={12}
                placeholder="<h2>Contrato de Prestação de Serviços</h2><p>Eu, {{PATIENT_NAME}}...</p>"
              />
              <button className="btn" type="submit">Criar template</button>
            </form>
          </div>

          <div className="card">
            <h3>Templates existentes</h3>
            {(templates || []).length === 0 ? <p className="muted">Nenhum template criado.</p> : (
              <table>
                <thead><tr><th>Nome</th><th>Criado em</th></tr></thead>
                <tbody>
                  {templates.map(t => (
                    <tr key={t.id}>
                      <td>{t.name}</td>
                      <td>{new Date(t.created_at).toLocaleDateString('pt-BR')}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </>
      )}
    </div>
  )
}
