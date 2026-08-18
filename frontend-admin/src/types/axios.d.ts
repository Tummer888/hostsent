import type { AxiosRequestConfig } from 'axios';

export interface Result<T = any> {
  code: number;
  data: T;
  message?: string;
  msg?: string;
}

export interface AxiosRequestConfigRetry extends AxiosRequestConfig {
  retryCount?: number;
}

export interface RequestOptions {
  apiUrl?: string;
  urlPrefix?: string;
  isJoinPrefix?: boolean;
  isTransformResponse?: boolean;
  isReturnNativeResponse?: boolean;
  joinParamsToUrl?: boolean;
  formatDate?: boolean;
  joinTime?: boolean;
  ignoreCancelToken?: boolean;
  withToken?: boolean;
  retry?: {
    count: number;
    delay: number;
  };
  throttle?: {
    delay: number;
  };
  debounce?: {
    delay: number;
  };
}
