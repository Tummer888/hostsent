import { createApp } from 'vue'
import TDesign from 'tdesign-vue-next'
import 'tdesign-vue-next/es/style/index.css'

import App from './App.vue'
import { setupPermission } from './permission'
import router from './router'
import { pinia } from './store'
import './styles/index.css'

const app = createApp(App)
app.use(pinia)
app.use(router)
app.use(TDesign)
setupPermission(app)
app.mount('#app')
