import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 使用 Vite 原生支持的 import.meta.url，避免依赖 node 类型
const base = new URL('.', import.meta.url).pathname
function resolve(p: string) {
  // 处理 Windows 盘符前缀问题（Linux 下无需处理）
  return `${base.replace(/\/$/, '')}/${p.replace(/^\.\//, '')}`
}

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve('./src'),
    },
  },
  server: {
    host: '127.0.0.1',
    port: 3002,
    proxy: {
      '/api/v1': {
        target: 'http://127.0.0.1:18080',
        changeOrigin: true,
      },
    },
  },
})
