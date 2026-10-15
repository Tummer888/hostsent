import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

// 首屏慢的两个根因（与 frontend-admin 完全同构，实测于本仓库 Vite 8.3.4）：
//
// 一、预构建依赖的 sourcemap 被内联进响应。
//   预构建时 Vite 硬编码 `sourcemap: "hidden"`（写在 spread 之后，配置项覆盖不掉），
//   在 node_modules/.vite/deps 旁边生成同名 .map（图标那个 chunk 的 .map 有 21 MB）；
//   dev 响应时发送层把该 .map 以 base64 内联成 `//# sourceMappingURL=data:...`
//   追加进 JS。base64 膨胀 1/3，第三方依赖被整体放大约 3.4 倍：
//     esm-*.js（tdesign 图标全量 barrel）磁盘 11.70 MB → 传输 39.96 MB
//     tdesign-vue-next.js                     磁盘  2.36 MB → 传输  8.97 MB
//   注意不能直接删 .map：发送层有兜底分支，拿不到 map 时会现场生成 includeContent
//   的自映射，反而把整份代码再 base64 一遍（实测删掉后单文件涨到 64.79 MB）。
//   所以写入 mappings 为空的桩 map，两个分支都跳过。
//
// 二、dev 响应不做传输压缩，上述字节原样落到网上。
//   实测 /login 首屏网线流量 53.54 MB，其中 49 MB 是上面两个依赖。
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

// 必须 await 完再让 configureServer 返回：中间件栈先到先服务，把 use() 拖到
// 下一个微任务就会排在 Vite 内置静态中间件之后，永远拿不到响应。
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
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 3002,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        // 同 frontend-admin：让后端拿到浏览器真实来源地址而不是 dev server 地址。
        xfwd: true,
      },
    },
  },
})
