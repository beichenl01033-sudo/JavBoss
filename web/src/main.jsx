import React from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from '@/App'
import AppThemeProvider from '@/app/theme/AppThemeProvider'
import { AuthProvider } from '@/features/auth/components/AuthProvider'
import { LtxCatalogProvider } from '@/features/ltx/components/LtxBadges'
import '@/index.css'

createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <AppThemeProvider>
      <BrowserRouter>
        <AuthProvider>
          <LtxCatalogProvider>
            <App />
          </LtxCatalogProvider>
        </AuthProvider>
      </BrowserRouter>
    </AppThemeProvider>
  </React.StrictMode>
)
