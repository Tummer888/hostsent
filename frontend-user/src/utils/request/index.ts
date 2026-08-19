import axios, { type AxiosInstance, type AxiosRequestConfig, type InternalAxiosRequestConfig } from 'axios'
import { MessagePlugin } from 'tdesign-vue-next'
import { useUserStore } from '@/store'

const request: AxiosInstance = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://127.0.0.1:8080/api/v1',
  timeout: 30000,
})

request.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const userStore = useUserStore()
    if (userStore.token) {
      config.headers.Authorization = `Bearer ${userStore.token}`
    }
    return config
  },
  (error) => Promise.reject(error)
)

request.interceptors.response.use(
  (response) => {
    if (response.config.responseType === 'blob') {
      return response
    }
    
    const { code, data, message } = response.data
    
    if (code === 0 || code === 200) {
      return response.data
    }
    
    if (code === 401) {
      const userStore = useUserStore()
      userStore.logout()
      MessagePlugin.error('登录已过期，请重新登录')
      window.location.href = '/login'
      return Promise.reject(new Error(message || '未授权'))
    }
    
    MessagePlugin.error(message || '请求失败')
    return Promise.reject(new Error(message || '请求失败'))
  },
  (error) => {
    if (error.response) {
      const { status, data } = error.response
      
      if (status === 401) {
        const userStore = useUserStore()
        userStore.logout()
        MessagePlugin.error('登录已过期，请重新登录')
        window.location.href = '/login'
      } else {
        const message = data?.message || error.message
        MessagePlugin.error(message)
      }
    } else {
      MessagePlugin.error('网络连接异常，请检查网络')
    }
    return Promise.reject(error)
  }
)

export default request
