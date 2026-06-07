import { useEffect, useState } from 'react'
import { psychApi } from '../api'

function formatBRL(cents) {
  return (cents / 100).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}

export default function PsychFinance() {
  const now = new Date()
  const [summary, setSummary] = useState(null)
  const [reports, setReports] = useState([])
  const [error, setError] = useState('')

  useEffect(() => {
    const m = now.getMonth() + 1, y = now.getFullYear()
    Promise.all([psychApi.financeSummary(m, y), psychApi.monthlyReports(m, y)])
      .then(([s, r]) => { setSummary(s); setReports(r) })
      .catch((e) => setError(e.message))
  }, [])

  return (
    <div>
      <h2>Financeiro — {now.toLocaleString('pt-BR', { month: 'long', year: 'numeric' })}</h2>
      {error && <p className="error">{error}</p>}

      {summary && (
        <div className="grid grid-2">
          <div className="card"><h3>Recebido</h3><p>{formatBRL(summary.total_received)}</p></div>
          <div className="card"><h3>A receber</h3><p>{formatBRL(summary.total_pending)}</p></div>
          <div className="card"><h3>Custos</h3><p>{formatBRL(summary.total_costs)}</p></div>
          <div className="card"><h3>Saldo</h3><p>{formatBRL(summary.balance)}</p></div>
        </div>
      )}

      <div className="card">
        <h3>Relatório mensal por paciente</h3>
        {reports.length === 0 ? <p className="muted">Sem dados.</p> : (
          <table>
            <thead><tr><th>Paciente</th><th>Consultas</th><th>Valor</th></tr></thead>
            <tbody>
              {reports.map((r) => (
                <tr key={r.patient_id}>
                  <td>{r.patient_name}</td>
                  <td>{r.appointments?.length || 0}</td>
                  <td>{formatBRL(r.total_amount)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
