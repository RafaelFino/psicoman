import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { patientApi, devApi, savePatientToken, isDevMode } from '../api'

export default function PatientLogin() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const [form, setForm] = useState({ name: '', email: '', phone: '', anamnesis: '' })
  const [devForm, setDevForm] = useState({ name: '', email: '' })
  const [error, setError] = useState('')
  const [tab, setTab] = useState('login')
  const [devMode, setDevMode] = useState(false)
  const [devLoading, setDevLoading] = useState(false)

  // Capture token returned by Google OAuth callback redirect
  useEffect(() => {
    const token = params.get('token')
    if (token) {
      savePatientToken(token)
      navigate('/patient')
    }
  }, [params, navigate])

  // Detect dev mode once
  useEffect(() => {
    isDevMode().then(setDevMode)
  }, [])

  const googleLogin = async () => {
    try {
      const { url } = await patientApi.authUrl()
      window.location.href = url
    } catch (err) {
      setError(err.message)
    }
  }

  const register = async (e) => {
    e.preventDefault()
    try {
      await patientApi.register(form)
      setTab('login')
      setError('')
      alert('Cadastro realizado! Faça login com Google.')
    } catch (err) {
      setError(err.message)
    }
  }

  // Dev-only: create/reuse patient and log in without Google
  const devLogin = async (e) => {
    e.preventDefault()
    if (!devForm.name || !devForm.email) {
      setError('Nome e email são obrigatórios')
      return
    }
    setDevLoading(true)
    setError('')
    try {
      const data = await devApi.createPatient(devForm.name, devForm.email)
      if (data.error) { setError(data.error); return }
      savePatientToken(data.token)
      navigate('/patient')
    } catch (err) {
      setError(err.message)
    } finally {
      setDevLoading(false)
    }
  }

  return (
    <div className="container" style={{ maxWidth: 480, marginTop: '3rem' }}>
      <div className="card">
        <h2>Área do Paciente</h2>
        {error && <p className="error">{error}</p>}

        <div className="nav" style={{ marginBottom: '1rem' }}>
          <button
            className={`btn ${tab === 'login' ? '' : 'secondary'}`}
            onClick={() => setTab('login')}
          >
            Entrar
          </button>
          <button
            className={`btn ${tab === 'register' ? '' : 'secondary'}`}
            onClick={() => setTab('register')}
          >
            Cadastrar
          </button>
          {devMode && (
            <button
              className={`btn ${tab === 'dev' ? '' : 'secondary'}`}
              onClick={() => setTab('dev')}
              title="Disponível apenas em DEV_MODE"
            >
              🛠 Dev
            </button>
          )}
        </div>

        {tab === 'login' && (
          <>
            <p className="muted">
              Entre com sua conta Google para acessar seus atendimentos.
            </p>
            <button className="btn" onClick={googleLogin}>
              Entrar com Google
            </button>
          </>
        )}

        {tab === 'register' && (
          <form onSubmit={register}>
            <label>Nome</label>
            <input
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              required
            />
            <label>Email</label>
            <input
              type="email"
              value={form.email}
              onChange={(e) => setForm({ ...form, email: e.target.value })}
              required
            />
            <label>Telefone</label>
            <input
              value={form.phone}
              onChange={(e) => setForm({ ...form, phone: e.target.value })}
            />
            <label>Anamnese inicial</label>
            <textarea
              value={form.anamnesis}
              onChange={(e) => setForm({ ...form, anamnesis: e.target.value })}
              rows={5}
            />
            <button className="btn" type="submit">
              Criar conta
            </button>
          </form>
        )}

        {tab === 'dev' && devMode && (
          <form onSubmit={devLogin}>
            <div
              style={{
                background: '#fef9c3',
                border: '1px solid #fde68a',
                borderRadius: 8,
                padding: '0.75rem',
                marginBottom: '1rem',
                fontSize: '0.9rem',
              }}
            >
              <strong>Modo desenvolvimento</strong> — login sem Google OAuth.
              Um paciente será criado ou reutilizado com esse email.
            </div>
            <label>Nome</label>
            <input
              value={devForm.name}
              onChange={(e) => setDevForm({ ...devForm, name: e.target.value })}
              placeholder="Paciente Teste"
              required
            />
            <label>Email</label>
            <input
              type="email"
              value={devForm.email}
              onChange={(e) => setDevForm({ ...devForm, email: e.target.value })}
              placeholder="paciente@teste.local"
              required
            />
            <button className="btn" type="submit" disabled={devLoading}>
              {devLoading ? 'Entrando...' : 'Entrar sem Google'}
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
