import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'

export const inputClass = 'w-full border border-gray-200 rounded-lg px-3 py-2.5 focus:outline-none focus:ring-2 focus:ring-brand-500 disabled:bg-gray-50'
export const buttonClass = 'w-full bg-brand-500 hover:bg-brand-600 text-white rounded-lg py-2.5 font-medium disabled:opacity-50 disabled:cursor-not-allowed'
export default function AuthLayout({ title, subtitle, children }: { title: string; subtitle: string; children: ReactNode }) {
  return <main className="min-h-screen flex items-center justify-center bg-white font-sans px-5 py-12">
    <section className="w-full max-w-sm space-y-6">
      <Link to="/" className="inline-block text-2xl font-serif text-brand-600">行迹 Wandr</Link>
      <div><h1 className="text-2xl font-medium text-gray-900">{title}</h1><p className="mt-2 text-sm leading-6 text-gray-500">{subtitle}</p></div>
      {children}
      <Link to="/" className="block text-center text-sm text-gray-400 hover:text-brand-600">返回首页</Link>
    </section>
  </main>
}
