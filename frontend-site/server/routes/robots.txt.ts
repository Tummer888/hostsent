/**
 * robots.txt。
 *
 * 用服务端路由而非静态文件，是为了拼出绝对地址的 Sitemap 行
 * （站点域名由部署环境决定，静态文件无法感知）。
 */
export default defineEventHandler((event) => {
  const { public: publicConfig } = useRuntimeConfig(event)
  const origin = (publicConfig.siteUrl || getRequestURL(event).origin).replace(/\/+$/, '')

  setHeader(event, 'content-type', 'text/plain; charset=utf-8')
  setHeader(event, 'cache-control', 'public, max-age=3600')

  return [
    'User-Agent: *',
    'Allow: /',
    // 控制台/账户相关路径不应被收录
    'Disallow: /console',
    `Sitemap: ${origin}/sitemap.xml`,
    '',
  ].join('\n')
})
