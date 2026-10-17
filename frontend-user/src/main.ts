import { createApp } from 'vue'
import 'tdesign-vue-next/es/style/index.css'

import App from './App.vue'
import { permission } from './directives/permission'
import { setupPermission } from './permission'
import { setupTDesign } from './plugins/tdesign'
import router from './router'
import { pinia, useSettingsStore } from './store'
import './styles/index.css'
// 设计 Token 与页面骨架契约（对齐 frontend-admin 的 --hs-* / surface-card 体系）。
// 必须放在 index.css 之后：tokens.css 覆盖了同名的 --space-* 一类变量。
import './styles/tokens.css'
import './styles/console-module.css'

// 代登录（管理端新窗口带入 ?token=）：写入登录态并清除 URL 参数，
// 供 pinia store 初始化与请求拦截器读取，使新窗口免登录直接进入用户端。
const urlParams = new URLSearchParams(window.location.search)
const urlToken = urlParams.get('token')
if (urlToken) {
  localStorage.setItem('user_token', urlToken)
  window.history.replaceState({}, '', window.location.pathname + window.location.hash)
}

const app = createApp(App)
app.use(pinia)
// 用户端主题设置不依赖路由/登录态，先应用可避免首屏闪烁
useSettingsStore().init()
app.use(router)
// 按需注册（见 plugins/tdesign.ts）：替代全量 app.use(TDesign)，首屏少下一大批未用组件。
setupTDesign(app)
app.directive('permission', permission)
setupPermission(app)

// 渲染期异常兜底：没有它，组件 render/setup 抛错时只会在控制台留一条日志，
// 用户看到的是停在半截的页面（或白屏），既不知道发生了什么也没有出口。
app.config.errorHandler = (error, _instance, info) => {
  console.error('[user] 未捕获的渲染异常:', error, info)
  const current = router.currentRoute.value
  if (current.name === 'ServerError') return
  void router.replace({ name: 'ServerError', query: { from: current.fullPath } })
}

app.mount('#app')
