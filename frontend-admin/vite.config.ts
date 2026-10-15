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

// 首屏慢的元凶：预构建依赖的 sourcemap 被内联进响应。
//
// 因果链（实测于本仓库，Vite 8.3.4）：
//   1) 预构建依赖时 Vite 硬编码 `sourcemap: "hidden"`（写在 spread 之后，所以
//      optimizeDeps.rolldownOptions.output.sourcemap = false 会被覆盖），在
//      node_modules/.vite/deps 旁边生成同名 .map —— tdesign 图标那个 chunk 的
//      .map 有 21 MB；
//   2) dev 响应时 `vite:optimized-deps` 把 .map 读出来交给发送层，发送层以
//      base64 内联成 `//# sourceMappingURL=data:...` 追加进 JS。base64 膨胀 1/3：
//        esm-*.js（tdesign 图标全量 barrel）磁盘 11.70 MB → 传输 39.96 MB
//        tdesign-vue-next.js                     磁盘  2.36 MB → 传输  8.97 MB
//   3) dev 响应不做传输压缩。实测 /dashboard/base 首屏网线流量 53.42 MB。
//
// 注意：不能直接删掉 .map。发送层有个兜底分支（server/send.ts）——当拿不到
// map、或 map.mappings 非空时，它会用 MagicString 现场生成一份 includeContent
// 的自映射，反而把整份代码再 base64 一遍（实测删掉 .map 后单文件涨到 64.79 MB）。
// 所以这里写入一个 mappings 为空的桩 map：两个分支都因此跳过，响应里既不带
// 内联 map，也不触发兜底生成。代价仅是「不能在 node_modules 内部单步调试第三方
// 源码」，自有 src/** 的 sourcemap 不受影响。
function stubDepSourcemaps() {
  return {
    name: 'hostsent:stub-dep-sourcemaps',
    generateBundle(_options: unknown, bundle: Record<string, unknown>) {
      for (const [fileName, item] of Object.entries(bundle)) {
        if (!fileName.endsWith('.map')) continue
        const target = item as { source?: unknown; type?: string }
        if (target.type !== 'asset') continue
        target.source = JSON.stringify({
          version: 3,
          file: fileName.replace(/\.map$/, ''),
          sources: [],
          sourcesContent: [],
          names: [],
          mappings: '',
        })
      }
    },
  }
}

// dev / preview 响应压缩。两者默认都不做传输压缩，而预构建依赖动辄十几 MB，
// 压缩后能降到约 1/4（图标 chunk：11.70 MB → gzip 约 2.8 MB）。
//
// 必须 await 完再让 configureServer 返回：中间件栈是先到先服务，若把 use() 拖到
// 下一个微任务，压缩会排在 Vite 内置静态中间件之后，永远拿不到响应。
async function attachCompression(middlewares: { use: (fn: unknown) => void }) {
  const { default: compression } = await import('compression')
  middlewares.use(
    compression({
      // 小于 1KB 压缩收益为负
      threshold: 1024,
      // 已是压缩格式的（图片/字体）不重复压缩
      filter: (
        req: { headers: Record<string, string | string[] | undefined> },
        res: { getHeader: (k: string) => unknown },
      ) => {
        if (req.headers['x-no-compression']) return false
        const type = String(res.getHeader('content-type') || '')
        return !/^(image|font)\//.test(type) && !/woff2?|\.png$|\.jpe?g$|\.gif$|\.webp$|\.ico$/.test(type)
      },
    }) as unknown,
  )
}

export default defineConfig({
  optimizeDeps: {
    rolldownOptions: {
      plugins: [stubDepSourcemaps()],
    },
  },
  plugins: [
    vue(),
    {
      name: 'hostsent:dev-compression',
      async configureServer(server) {
        await attachCompression(server.middlewares)
      },
      async configurePreviewServer(server) {
        await attachCompression(server.middlewares)
      },
    },
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
    alias: [
      // 图标包收窄：把裸包名指到「按需生成的入口」，只转发项目与 tdesign 实际
      // import 的 171 个图标，而不是 barrel 里的 2354 个。不这样做的话，dev 预构建
      // 的 esm-*.js 恒为 11.70 MB（实测传输 914ms + 浏览器编译 669ms）。
      // 必须用精确匹配数组形式：写成对象键会把本入口内部对
      // 'tdesign-icons-vue-next/esm/components/xxx.js' 的深路径引用一起劫持，
      // 造成无限自引用。生成脚本见 scripts/gen-tdesign-icons.mjs（predev/prebuild 自动跑）。
      { find: '@', replacement: resolve('./src') },
    ],
  },
  server: {
    host: '0.0.0.0',
    port: 3000,
    allowedHosts: true,
    proxy: {
      '/api/v1': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        // 转发真实客户端 IP：后端只在「直连对端可信」时才采信 X-Forwarded-For
        // （见 netutil.ClientIP / app.trusted_proxies），不打开 xfwd 的话后端
        // 只能看到 dev server 自己的地址，登录日志里所有用户都是同一个 IP。
        xfwd: true,
      },
    },
  },
  // 生产构建的本地预演：preview 不继承 server.proxy，需单独声明，
  // 否则用构建产物核对首屏时 /api/v1 全部 404。
  preview: {
    port: 3100,
    proxy: {
      '/api/v1': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        xfwd: true,
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
