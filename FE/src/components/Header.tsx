import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../store/auth'
import { useLogout } from '../api/auth'

export default function Header({ onStartWizard }: { onStartWizard?: () => void }) {
  const { isAuthed, user } = useAuth()
  const logout = useLogout()
  const nav = useNavigate()

  const onPill = () => {
    if (!isAuthed) { nav('/login'); return }
    onStartWizard?.()
  }

  return (
    <header className="fixed top-0 inset-x-0 z-50 bg-white/90 backdrop-blur border-b border-gray-100">
      <div className="mx-auto max-w-container px-4 h-16 flex items-center justify-between">
        <Link to="/" className="text-xl font-serif text-brand-600">行迹 Wandr</Link>

        <button
          onClick={onPill}
          className="hidden sm:flex items-center gap-2 rounded-full border border-gray-200 shadow-sm px-4 py-2 text-sm text-gray-600 hover:shadow-md transition"
        >
          <span>想去哪儿？说说你的偏好</span>
          <span className="rounded-full bg-brand-500 text-white text-xs px-2 py-0.5">一键生成</span>
        </button>

        <nav className="flex items-center gap-3 text-sm">
          {isAuthed ? (
            <>
              <Link to="/places" className="text-gray-700 hover:text-brand-600">发现地点</Link>
              <Link to="/trips" className="text-gray-700 hover:text-brand-600">我的行程</Link>
              {user?.role === 'admin' && (
                <Link to="/admin/places" className="text-gray-700 hover:text-brand-600">景点维护</Link>
              )}
              {user?.username && <span title={user.email || user.username} className="hidden sm:inline-block max-w-40 truncate text-gray-400">你好，{user.email || user.username}</span>}
              <button disabled={logout.isPending} onClick={() => logout.mutate()} className="text-gray-500 hover:text-brand-600 disabled:opacity-50">{logout.isPending ? '退出中…' : '退出'}</button>
            </>
          ) : (
            <Link to="/login" className="rounded-lg bg-brand-500 hover:bg-brand-600 text-white px-3 py-1.5">登录</Link>
          )}
        </nav>
      </div>
    </header>
  )
}
