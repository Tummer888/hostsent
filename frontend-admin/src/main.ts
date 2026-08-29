import { createApp } from 'vue'
// TDesign 组件改为按需引入（见 vite.config.ts 的 unplugin-vue-components），
// 仅在此处保留基础样式与命令式 API（Message/Notify 等）样式，避免全量打包。
import 'tdesign-vue-next/es/style/index.css'
import 'tdesign-vue-next/es/message/style/index.css'
import 'tdesign-vue-next/es/notification/style/index.css'
import 'tdesign-vue-next/es/dialog/style/index.css'
import 'tdesign-vue-next/es/loading/style/index.css'

import App from './App.vue'
import { setupPermission } from './permission'
import router from './router'
import { pinia } from './store'
import './styles/index.css'

const app = createApp(App)
app.use(pinia)
app.use(router)
setupPermission(app)
app.mount('#app')
