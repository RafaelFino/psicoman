import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { patientApi, savePatientToken } from '../api'

export default function PatientLogin() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const [form, setForm] = useState({ name: '', email: '', phone: '', anamnesis: '' })
  const [error, setError] = useState('')
  const [tab, setTab] = useState('login')

  useEffect(() => {
    const token = params.get('token')
    if (token) {
      savePatientToken(token)
      navigate('/patient')
    }
  }, [params, navigate])

  const googleLogin = async () => {
    try {
      const { url } = await patientApi.authUrl()
      window.location.href = url
    } catch (err) { setError(err.message) }
  }

  const register = async (e) => {
    e.preventDefault()
    try {
      await patientApi.register(form)
      setTab('login')
      setError('')
      alert('Cadastro realizado! Faça login com Google.')
    } catch (err) { setError(err.message) }
  }

  return (
    <div className="container" style={{ maxWidth: 480, marginTop: '3rem' }}>
      <div className="card">
        <h2>Área do Paciente</h2>
        {error && <p className="error">{error}</p>}

        <div className="nav" style={{ marginBottom: '1rem' }}>
          <button className={`btn ${tab === 'login' ? '' : 'secondary'}`} onClick={() => setTab('login')}>Entrar</button>
          <button className={`btn ${tab === 'register' ? '' : 'secondary'}`} onClick={() => setTab('register')}>Cadastrar</button>
        </div>

        {tab === 'login' ? (
          <>
            <p className="muted">Entre com sua conta Google para acessar seus atendimentos.</p>
            <button className="btn" onClick={googleLogin}>Entrar com Google</button>
          </>
        ) : (
          <form onSubmit={register}>
            <label>Nome</label>
            <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
            <label>Email</label>
            <input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required />
            <label>Telefone</label>
            <input value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} />
            <label>Anamnese inicial</label>
            <textarea value={form.anamnesis} onChange={(e) => setForm({ ...form, anamnesis: e.target.value })} rows={5} />
            <button className="btn" type="submit">Criar conta</button>
          </form>
        )}
      </div>
    </div>
  )
}
