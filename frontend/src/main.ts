import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { router } from '@/router/index'
import '@/assets/main.css'
import App from './App.vue'

const app = createApp(App)
app.use(createPinia()) // pinia before router — beforeEach guard uses useUserStore()
app.use(router)
app.mount('#app')
