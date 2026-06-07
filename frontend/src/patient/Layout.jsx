import { NavLink, Outlet, Navigate } from 'react-router-dom'

const links = [
  ['/patient', 'Meus atendimentos'],
  ['/patient/book', 'Agendar'],
  ['/patient/anamnesis', 'Anamnese'],
  ['/patient/documents', 'Documentos'],
]

export default function PatientLayout() {
  if (!localStorage.getItem('patient_token')) {
    return <Navigate to="/patient/login" replace />
  }

  return (
    <>
      <header className="header">
        <div className="container">
          <h1>Psicoman — Paciente</h1>
          <nav className="nav">
            {links.map(([to, label]) => (
              <NavLink key={to} to={to} end={to === '/patient'}>{label}</NavLink>
            ))}
          </nav>
        </div>
      </header>
      <main className="container">
        <Outlet />
      </main>
    </>
  )
}
