// ─── Dev mode detection ──────────────────────────────────────────────────────
// When the server is running with DEV_MODE=true, /api/dev/status returns 200.
// We cache this result so it's only fetched once per page load.
let _devMode = null
let _devSecret = 'dev-local' // must match DEV_SECRET env var

export async function isDevMode() {
  if (_devMode !== null) return _devMode
  try {
    const res = await fetch('/api/dev/status')
    _devMode = res.ok
  } catch {
    _devMode = false
  }
  return _devMode
}

// Call this once at startup (main.jsx) to pre-warm the cache.
export function initDevMode() {
  isDevMode()
}

const patientToken = () => localStorage.getItem('patient_token')

async function request(path, options = {}) {
  const headers = { ...options.headers }

  if (options.patient) {
    const token = patientToken()
    if (token) headers['Authorization'] = `Bearer ${token}`
  }

  // Psych routes: attach X-Dev-Auth when dev mode is active
  if (options.psych && _devMode) {
    headers['X-Dev-Auth'] = _devSecret
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

// ─── Psych API ────────────────────────────────────────────────────────────────
export const psychApi = {
  me:                 ()       => request('/psych/me',                          { psych: true }),
  patients:           ()       => request('/psych/patients',                    { psych: true }),
  createPatient:      (data)   => request('/psych/patients',                    { method: 'POST', body: data, psych: true }),
  getPatient:         (id)     => request(`/psych/patients/${id}`,              { psych: true }),
  appointments:       (params) => request(`/psych/appointments${params || ''}`, { psych: true }),
  createAppointment:  (data)   => request('/psych/appointments',                { method: 'POST', body: data, psych: true }),
  cancelAppointment:  (id, r)  => request(`/psych/appointments/${id}/cancel`,   { method: 'PATCH', body: { reason: r }, psych: true }),
  rescheduleAppointment: (id, at) => request(`/psych/appointments/${id}/reschedule`, { method: 'PATCH', body: { scheduled_at: at }, psych: true }),
  updateNotes:        (id, d)  => request(`/psych/appointments/${id}/notes`,    { method: 'PATCH', body: d, psych: true }),
  completeAppointment:(id)     => request(`/psych/appointments/${id}/complete`, { method: 'PATCH', psych: true }),
  rules:              ()       => request('/psych/scheduling-rules',            { psych: true }),
  updateRules:        (data)   => request('/psych/scheduling-rules',            { method: 'PUT', body: data, psych: true }),
  documents:          (pid)    => request(`/psych/documents?patient_id=${pid || ''}`, { psych: true }),
  uploadDocument:     (form)   => request('/psych/documents',                  { method: 'POST', body: form, psych: true }),
  financeSummary:     (m, y)   => request(`/psych/finance/summary?month=${m}&year=${y}`, { psych: true }),
  monthlyReports:     (m, y)   => request(`/psych/finance/reports/monthly?month=${m}&year=${y}`, { psych: true }),
  addPayment:         (data)   => request('/psych/finance/payments',            { method: 'POST', body: data, psych: true }),
  receivePayment:     (id)     => request(`/psych/finance/payments/${id}/receive`, { method: 'POST', psych: true }),
  addCost:            (data)   => request('/psych/finance/costs',               { method: 'POST', body: data, psych: true }),
  patientReport:      (id)     => request(`/psych/patients/${id}/report`,      { psych: true }),
}

// ─── Patient API ──────────────────────────────────────────────────────────────
export const patientApi = {
  me:           ()     => request('/patient/me',           { patient: true }),
  appointments: ()     => request('/patient/appointments', { patient: true }),
  slots:        ()     => request('/patient/slots',        { patient: true }),
  book:         (data) => request('/patient/appointments', { method: 'POST', body: data, patient: true }),
  cancel:       (id, reason) => request(`/patient/appointments/${id}/cancel`, { method: 'PATCH', body: { reason }, patient: true }),
  reschedule:   (id, at)     => request(`/patient/appointments/${id}/reschedule`, { method: 'PATCH', body: { scheduled_at: at }, patient: true }),
  anamnesis:    (data) => request('/patient/anamnesis',    { method: 'PUT',  body: data, patient: true }),
  documents:    ()     => request('/patient/documents',    { patient: true }),
  authUrl:      ()     => request('/auth/patient/url'),
  register:     (data) => request('/auth/patient/register', { method: 'POST', body: data }),
}

// ─── Dev API ──────────────────────────────────────────────────────────────────
export const devApi = {
  // Creates a patient (or reuses existing) and returns a ready-to-use JWT.
  createPatient: (name, email) =>
    fetch('/api/dev/create-patient', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Dev-Auth': _devSecret },
      body: JSON.stringify({ name, email }),
    }).then((r) => r.json()),
}

export function savePatientToken(token) {
  localStorage.setItem('patient_token', token)
}
