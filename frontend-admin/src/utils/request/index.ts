import {
  AxiosError,
  type AxiosInstance,
  type AxiosRequestConfig,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from 'axios'
import axios from 'axios'

import router from '@/router'
import { useUserStore } from '@/store'
import type { Result } from '@/types/axios'

declare module 'axios' {
  interface AxiosRequestConfig {
    _skipResultUnwrap?: boolean
  }
}

const URL_PREFIX = '/api/v1'

const instance: AxiosInstance = axios.create({
  baseURL: URL_PREFIX,
  timeout: 30_000,
  headers: {
    'Content-Type': 'application/json; charset=utf-8',
  },
})

const STORAGE_REDIRECT_KEY = 'hostsent_admin_last_path'

const getBackMessage = (data: unknown): string | undefined => {
  if (data && typeof data === 'object') {
    const payload = data as { message?: string; msg?: string }
    return payload.message || payload.msg
  }
  return undefined
}

instance.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const userStore = useUserStore()
  if (userStore.token && config.headers) {
    config.headers.Authorization = `Bearer ${userStore.token}`
  }
  const method = (config.method || 'GET').toUpperCase()
  if (method !== 'GET' && router.currentRoute.value.fullPath) {
    try {
      sessionStorage.setItem(STORAGE_REDIRECT_KEY, router.currentRoute.value.fullPath)
    } catch {
      /* ignore storage errors */
    }
  }
  return config
})

instance.interceptors.response.use(
  (response: AxiosResponse) => {
    const { data, config } = response
    if (config?._skipResultUnwrap) {
      return response as never
    }
    if (data && typeof data === 'object' && 'code' in data) {
      const result = data as Result
      const code = result.code ?? 0
      if (code === 0) {
        return result.data as never
      }
      const message = result.message || result.msg || `请求失败，错误码：${code}`
      return Promise.reject(new Error(message))
    }
    return data as never
  },
  (error: AxiosError) => {
    const status = error?.response?.status
    if (status === 401) {
      const userStore = useUserStore()
      userStore.logout()
      let redirect = ''
      try {
        redirect = sessionStorage.getItem(STORAGE_REDIRECT_KEY) || router.currentRoute.value.fullPath
      } catch {
        redirect = router.currentRoute.value.fullPath
      }
      router.replace({
        path: '/login',
        query: redirect ? { redirect: encodeURIComponent(redirect) } : undefined,
      })
    }
    const backendMessage = getBackMessage(error?.response?.data)
    if (backendMessage) {
      return Promise.reject(new Error(backendMessage))
    }
    const message =
      error?.code === 'ECONNABORTED'
        ? '请求超时，请稍后重试'
        : error?.code === 'ERR_NETWORK'
          ? '网络异常，请检查后端服务是否已启动'
          : error?.message || '请求失败，请稍后重试'
    return Promise.reject(new Error(message))
  },
)

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'

interface RequestOptions extends Omit<AxiosRequestConfig, 'method' | 'params' | 'data' | 'url'> {
  method?: Method
  params?: Record<string, unknown>
  data?: unknown
  url: string
}

function createRequest<T = unknown>(
  method: Method,
  url: string,
  data?: unknown,
  config?: AxiosRequestConfig,
): Promise<T> {
  const requestConfig: AxiosRequestConfig = {
    url,
    method,
    ...config,
  }
  if (method === 'GET') {
    requestConfig.params = data
  } else {
    requestConfig.data = data
  }
  return instance.request(requestConfig) as unknown as Promise<T>
}

export const request = {
  get<T = unknown>(options: RequestOptions): Promise<T> {
    return createRequest<T>('GET', options.url, options.params, options)
  },
  post<T = unknown>(options: RequestOptions): Promise<T> {
    return createRequest<T>('POST', options.url, options.data, options)
  },
  put<T = unknown>(options: RequestOptions): Promise<T> {
    return createRequest<T>('PUT', options.url, options.data, options)
  },
  delete<T = unknown>(options: RequestOptions): Promise<T> {
    return createRequest<T>('DELETE', options.url, options.data, options)
  },
  patch<T = unknown>(options: RequestOptions): Promise<T> {
    return createRequest<T>('PATCH', options.url, options.data, options)
  },
}
