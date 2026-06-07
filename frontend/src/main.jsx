import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import { initDevMode } from './api'
import './styles.css'

// Pre-warm dev mode detection so the first psych/patient request already
// knows whether to attach X-Dev-Auth. Fire-and-forget is fine here.
initDevMode()

ReactDOM.createRoot(document.getElementById('root')).render(
  <BrowserRouter>
    <App />
  </BrowserRouter>
)
