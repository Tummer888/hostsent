import { createApp } from 'vue'
import TDesign from 'tdesign-vue-next'
import 'tdesign-vue-next/es/style/index.css'

import App from './App.vue'
import { permission } from './directives/permission'
import { setupPermission } from './permission'
import router from './router'
import { pinia, useSettingsStore } from './store'
import './styles/index.css'

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
app.use(TDesign)
app.directive('permission', permission)
setupPermission(app)
app.mount('#app')
