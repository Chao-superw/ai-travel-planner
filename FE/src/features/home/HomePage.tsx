import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Header from '../../components/Header'
import { useAuth } from '../../store/auth'
import { TEMPLATES, QUICK_INTERESTS, type TripTemplate } from './templates'
import WizardModal from '../wizard/WizardModal'

function yuan(cents: number) {
  return (cents / 100).toFixed(0)
}

export default function HomePage() {
  const { isAuthed } = useAuth()
  const nav = useNavigate()
  const [active, setActive] = useState<string>('全部')
  const [wizardOpen, setWizardOpen] = useState(false)
  const [prefill, setPrefill] = useState<Partial<TripTemplate> | undefined>(undefined)

  const startWizard = (p?: Partial<TripTemplate>) => {
    if (!isAuthed) { nav('/login'); return }
    setPrefill(p)
    setWizardOpen(true)
  }

  const list = active === '全部' ? TEMPLATES : TEMPLATES.filter((t) => t.interests.includes(active))

  return (
    <div className="min-h-screen bg-white font-sans">
      <Header onStartWizard={() => startWizard()} />

      <main className="pt-16">
        {/* Hero */}
        <section className="mx-auto max-w-container px-4 py-14 text-center">
          <h1 className="text-3xl sm:text-4xl font-serif text-gray-900">和「行迹 Wandr」一起，把想去的地方变成行程</h1>
          <p className="mt-3 text-gray-500">说出预算、天数和喜好，AI 为你排好每一天的路线与花费。</p>
          <button
            onClick={() => startWizard()}
            className="mt-6 rounded-full bg-brand-500 hover:bg-brand-600 text-white px-6 py-3 font-medium shadow-sm"
          >
            开始规划我的旅行
          </button>
        </section>

        {/* 快速偏好横滑条 */}
        <section className="mx-auto max-w-container px-4">
          <div className="flex gap-2 overflow-x-auto pb-2">
            {QUICK_INTERESTS.map((i) => (
              <button
                key={i}
                onClick={() => setActive(i)}
                className={`shrink-0 rounded-full border px-4 py-1.5 text-sm transition ${
                  active === i ? 'border-brand-500 text-brand-600 bg-brand-50' : 'border-gray-200 text-gray-600 hover:border-gray-300'
                }`}
              >
                {i}
              </button>
            ))}
          </div>
        </section>

        {/* 模板卡片网格 */}
        <section className="mx-auto max-w-container px-4 py-8">
          <h2 className="text-lg font-medium text-gray-900 mb-4">热门线路</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
            {list.map((t) => (
              <button
                key={t.city + t.theme}
                onClick={() => startWizard(t)}
                className="text-left group rounded-2xl overflow-hidden border border-gray-100 hover:shadow-lg transition"
              >
                <div className="aspect-square overflow-hidden bg-gray-50">
                  <img src={t.img} alt={t.theme} loading="lazy" className="w-full h-full object-cover group-hover:scale-105 transition" />
                </div>
                <div className="p-4">
                  <div className="flex items-center justify-between">
                    <span className="font-medium text-gray-900">{t.city}</span>
                    <span className="text-xs text-gray-400">{t.region}</span>
                  </div>
                  <div className="mt-1 text-sm text-gray-600">{t.theme} · {t.days} 天</div>
                  <div className="mt-2 text-sm text-brand-600">预算参考 ¥{yuan(t.budget_cents)}/人</div>
                </div>
              </button>
            ))}
          </div>
        </section>
      </main>

      <WizardModal open={wizardOpen} onClose={() => setWizardOpen(false)} prefill={prefill} />
    </div>
  )
}
