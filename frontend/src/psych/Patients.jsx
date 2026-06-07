import { useEffect, useState } from 'react'
import { psychApi } from '../api'

export default function PsychPatients() {
  const [patients, setPatients] = useState([])
  const [form, setForm] = useState({ name: '', email: '', phone: '' })
  const [error, setError] = useState('')

  const load = () => psychApi.patients().then(setPatients).catch((e) => setError(e.message))
  useEffect(() => { load() }, [])

  const submit = async (e) => {
    e.preventDefault()
    try {
      await psychApi.createPatient(form)
      setForm({ name: '', email: '', phone: '' })
      load()
    } catch (err) {
      setError(err.message)
    }
  }

  return (
    <div>
      <h2>Pacientes</h2>
      {error && <p className="error">{error}</p>}

      <form className="card" onSubmit={submit}>
        <h3>Novo paciente</h3>
        <label>Nome</label>
        <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
        <label>Email</label>
        <input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required />
        <label>Telefone</label>
        <input value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} />
        <button className="btn" type="submit">Cadastrar</button>
      </form>

      <div className="card">
        <table>
          <thead><tr><th>Nome</th><th>Email</th><th>Telefone</th></tr></thead>
          <tbody>
            {patients.map((p) => (
              <tr key={p.id}><td>{p.name}</td><td>{p.email}</td><td>{p.phone}</td></tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
