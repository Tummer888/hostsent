import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import { TDesignResolver } from 'unplugin-vue-components/resolvers'

// 使用 Vite 原生支持的 import.meta.url，避免依赖 node 类型
const base = new URL('.', import.meta.url).pathname
function resolve(p: string) {
  // 处理 Windows 盘符前缀问题（Linux 下无需处理）
  return `${base.replace(/\/$/, '')}/${p.replace(/^\.\//, '')}`
}

export default defineConfig({
  plugins: [
    vue(),
    Components({
      // 不生成 components.d.ts，模板组件类型沿用 TDesign 全局声明，
      // 避免按需引入暴露既有页面中大量的表格/标签类型不匹配问题。
      dts: false,
      dirs: [],
      resolvers: [
        // 修正 TDesignResolver 对某些子组件名的推导（包内实际导出名与 t- 前缀切名不一致）
        (name: string) => {
          const fixMap: Record<string, string> = {
            TStep: 'StepItem',
          }
          const target = fixMap[name]
          if (!target) return
          return { name: target, from: 'tdesign-vue-next' }
        },
        TDesignResolver({ library: 'vue-next' }),
      ],
    }),
  ],
  resolve: {
    alias: {
      '@': resolve('./src'),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 3000,
    allowedHosts: true,
    proxy: {
      '/api/v1': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        // 将 echarts / zrender 拆分到独立 vendor chunk，便于浏览器缓存，
        // 避免其体积落入主入口或随路由懒加载多次打包。
        manualChunks(id: string) {
          if (id.includes('node_modules/echarts') || id.includes('node_modules/zrender')) {
            return 'echarts'
          }
        },
      },
    },
    // echarts 体积较大，放宽单 chunk 体积上限，消除误报警告
    chunkSizeWarningLimit: 600,
  },
})
