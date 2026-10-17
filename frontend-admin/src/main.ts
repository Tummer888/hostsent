import { createApp } from 'vue'
// TDesign 组件改为按需引入（见 vite.config.ts 的 unplugin-vue-components），
// 仅在此处保留基础样式与命令式 API（Message/Notify 等）样式，避免全量打包。
import 'tdesign-vue-next/es/style/index.css'
import 'tdesign-vue-next/es/message/style/index.css'
import 'tdesign-vue-next/es/notification/style/index.css'
import 'tdesign-vue-next/es/dialog/style/index.css'
import 'tdesign-vue-next/es/loading/style/index.css'

import App from './App.vue'
import { permission as permissionDirective } from './directives/permission'
import { setupPermission } from './permission'
import router from './router'
import { pinia } from './store'
import './styles/index.css'

const app = createApp(App)
app.use(pinia)
app.use(router)
app.directive('permission', permissionDirective)
setupPermission(app)

// 渲染期异常兜底：没有它，组件 render/setup 抛错时 Vue 只会往控制台打一条日志，
// 用户看到的是停在半截的页面（或白屏），既不知道发生了什么也没有出口。
// 这里把异常导到 /error/500，保留骨架与「重新加载」的能力。
app.config.errorHandler = (error, _instance, info) => {
  console.error('[admin] 未捕获的渲染异常:', error, info)
  const current = router.currentRoute.value
  if (current.name === 'ServerError') return
  void router.replace({ name: 'ServerError', query: { from: current.fullPath } })
}

app.mount('#app')
