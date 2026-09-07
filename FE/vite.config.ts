import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    host: true,
    port: 5173,
    strictPort: true,
    // /api 反代到本地后端：前后端同源，穿透只需暴露 5173 一个端口，且无跨域问题
    proxy: { '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true } },
    // 允许 cloudflared 随机子域名（*.trycloudflare.com）访问；仅用于临时分享
    allowedHosts: true,
  },
})
