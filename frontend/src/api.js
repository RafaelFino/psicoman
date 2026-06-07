const patientToken = () => localStorage.getItem('patient_token')

async function request(path, options = {}) {
  const headers = { ...options.headers }
  if (options.patient) {
    const token = patientToken()
    if (token) headers['Authorization'] = `Bearer ${token}`
  }
  if (options.body && !(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json'
    options.body = JSON.stringify(options.body)
  }
  const res = await fetch(`/api${path}`, { ...options, headers })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || res.statusText)
  }
  if (res.status === 204) return null
  return res.json()
}

export const psychApi = {
  me: () => request('/psych/me'),
  patients: () => request('/psych/patients'),
  createPatient: (data) => request('/psych/patients', { method: 'POST', body: data }),
  appointments: (params = '') => request(`/psych/appointments${params}`),
  createAppointment: (data) => request('/psych/appointments', { method: 'POST', body: data }),
  cancelAppointment: (id, reason) => request(`/psych/appointments/${id}/cancel`, { method: 'PATCH', body: { reason } }),
  updateNotes: (id, data) => request(`/psych/appointments/${id}/notes`, { method: 'PATCH', body: data }),
  completeAppointment: (id) => request(`/psych/appointments/${id}/complete`, { method: 'PATCH' }),
  rules: () => request('/psych/scheduling-rules'),
  updateRules: (data) => request('/psych/scheduling-rules', { method: 'PUT', body: data }),
  documents: (patientId) => request(`/psych/documents?patient_id=${patientId || ''}`),
  financeSummary: (m, y) => request(`/psych/finance/summary?month=${m}&year=${y}`),
  monthlyReports: (m, y) => request(`/psych/finance/reports/monthly?month=${m}&year=${y}`),
  patientReport: (id) => request(`/psych/patients/${id}/report`),
}

export const patientApi = {
  me: () => request('/patient/me', { patient: true }),
  appointments: () => request('/patient/appointments', { patient: true }),
  slots: () => request('/patient/slots', { patient: true }),
  book: (data) => request('/patient/appointments', { method: 'POST', body: data, patient: true }),
  cancel: (id, reason) => request(`/patient/appointments/${id}/cancel`, { method: 'PATCH', body: { reason }, patient: true }),
  anamnesis: (data) => request('/patient/anamnesis', { method: 'PUT', body: data, patient: true }),
  documents: () => request('/patient/documents', { patient: true }),
  authUrl: () => request('/auth/patient/url'),
  register: (data) => request('/auth/patient/register', { method: 'POST', body: data }),
}

export function savePatientToken(token) {
  localStorage.setItem('patient_token', token)
}
