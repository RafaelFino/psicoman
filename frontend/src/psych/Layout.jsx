import { NavLink, Outlet } from 'react-router-dom'

const links = [
  ['/psych', 'Início'],
  ['/psych/patients', 'Pacientes'],
  ['/psych/appointments', 'Atendimentos'],
  ['/psych/finance', 'Financeiro'],
  ['/psych/settings', 'Configurações'],
]

export default function PsychLayout() {
  return (
    <>
      <header className="header">
        <div className="container">
          <h1>Psicoman — Psicólogo</h1>
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
