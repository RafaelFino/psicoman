import { NavLink, Outlet } from 'react-router-dom'

const links = [
  ['/psych', 'Agenda'],
  ['/psych/patients', 'Pacientes'],
  ['/psych/appointments', 'Atendimentos'],
  ['/psych/session-notes', 'Evoluções'],
  ['/psych/anamnesis', 'Anamnese'],
  ['/psych/contracts', 'Contratos'],
  ['/psych/finance', 'Financeiro'],
  ['/psych/settings', 'Config'],
]

export default function PsychLayout() {
  return (
    <>
      <header className="header">
        <div className="container">
          <h1>Psicoman</h1>
          <nav className="nav">
            {links.map(([to, label]) => (
              <NavLink key={to} to={to} end={to === '/psych'}>{label}</NavLink>
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
