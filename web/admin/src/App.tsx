import { NavLink, Navigate, Route, Routes } from 'react-router-dom'
import ClockPage from './pages/ClockPage'
import MigrationsPage from './pages/MigrationsPage'
import VersionPage from './pages/VersionPage'

export default function App() {
  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <header className="border-b border-slate-800 bg-slate-900">
        <div className="mx-auto flex max-w-5xl items-center gap-6 px-6 py-4">
          <h1 className="text-lg font-semibold">FNB Admin</h1>
          <nav className="flex gap-4 text-sm">
            <NavItem to="/clock">시간 이동</NavItem>
            <NavItem to="/version">버전 관리</NavItem>
            <NavItem to="/migrations">DB 마이그레이션</NavItem>
            {/* 새 페이지 추가 시 NavItem 한 줄 + Route 한 줄 */}
          </nav>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-6 py-8">
        <Routes>
          <Route path="/" element={<Navigate to="/clock" replace />} />
          <Route path="/clock" element={<ClockPage />} />
          <Route path="/version" element={<VersionPage />} />
          <Route path="/migrations" element={<MigrationsPage />} />
          <Route path="*" element={<p className="text-slate-400">Not Found</p>} />
        </Routes>
      </main>
    </div>
  )
}

function NavItem({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        `rounded px-2 py-1 transition ${
          isActive
            ? 'bg-slate-100 text-slate-900'
            : 'text-slate-400 hover:bg-slate-800 hover:text-slate-100'
        }`
      }
    >
      {children}
    </NavLink>
  )
}
