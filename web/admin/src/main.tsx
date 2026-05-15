import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App.tsx'
import './index.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    {/* Go 측 라우트가 /admin/ui/* 이므로 basename 일치 */}
    <BrowserRouter basename="/admin/ui">
      <App />
    </BrowserRouter>
  </StrictMode>,
)
