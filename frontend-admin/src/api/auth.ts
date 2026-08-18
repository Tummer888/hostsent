import { request } from '@/utils/request';

export interface LoginRequest {
  username: string;
  password: string;
}

export interface AuthUserInfo {
  id: number;
  username: string;
  role: string;
  roles: string[];
  email: string;
  phone: string;
  status: string;
}

export interface LoginResponse {
  token: string;
  user_info: AuthUserInfo;
  permissions: string[];
  menus: string[];
}

const Api = {
  login: '/auth/login',
  me: '/auth/me',
};

export function login(data: LoginRequest) {
  return request.post<LoginResponse>({
    url: Api.login,
    data,
  });
}

export function getCurrentUser() {
  return request.get<AuthUserInfo>({
    url: Api.me,
  });
}
