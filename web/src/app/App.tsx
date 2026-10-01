import type { ReactNode } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'
import { LoginPage } from '@/features/auth/LoginPage'
import { RegisterPage } from '@/features/auth/RegisterPage'
import { MessengerShell } from '@/features/shell/MessengerShell'

function Protected({ children }: { children: ReactNode }) {
  const token = useAuthStore((s) => s.accessToken)
  const hydrated = useAuthStore((s) => s.hydrated)
  if (!hydrated) return null
  if (!token) return <Navigate to="/login" replace />
  return children
}

export function App() {
  return (
    <div className="app-root">
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route
          path="/*"
          element={
            <Protected>
              <MessengerShell />
            </Protected>
          }
        />
      </Routes>
    </div>
  )
}
