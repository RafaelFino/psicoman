import { Routes, Route, Navigate } from 'react-router-dom'
import PsychLayout from './psych/Layout'
import PsychDashboard from './psych/Dashboard'
import PsychPatients from './psych/Patients'
import PsychAppointments from './psych/Appointments'
import PsychFinance from './psych/Finance'
import PsychSettings from './psych/Settings'
import PatientLayout from './patient/Layout'
import PatientLogin from './patient/Login'
import PatientDashboard from './patient/Dashboard'
import PatientBook from './patient/Book'
import PatientAnamnesis from './patient/Anamnesis'
import PatientDocuments from './patient/Documents'

export default function App() {
  return (
    <Routes>
      <Route path="/psych" element={<PsychLayout />}>
        <Route index element={<PsychDashboard />} />
        <Route path="patients" element={<PsychPatients />} />
        <Route path="appointments" element={<PsychAppointments />} />
        <Route path="finance" element={<PsychFinance />} />
        <Route path="settings" element={<PsychSettings />} />
      </Route>
      <Route path="/patient/login" element={<PatientLogin />} />
      <Route path="/patient" element={<PatientLayout />}>
        <Route index element={<PatientDashboard />} />
        <Route path="book" element={<PatientBook />} />
        <Route path="anamnesis" element={<PatientAnamnesis />} />
        <Route path="documents" element={<PatientDocuments />} />
      </Route>
      <Route path="*" element={<Navigate to="/psych" replace />} />
    </Routes>
  )
}
