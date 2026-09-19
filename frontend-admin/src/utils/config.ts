// 构建期注入的运行时配置集中在这里，避免同一个地址在多个页面各写一份兜底值。
// 可注入项见 frontend-admin/.env.example；生产环境应在构建时显式指定，
// 否则会回落到开发端口。

function normalizeBase(raw: string | undefined, fallback: string): string {
  const value = (raw ?? '').trim()
  return (value || fallback).replace(/\/+$/, '')
}

/**
 * 用户控制台（frontend-user）地址。管理端「代登录」需要在新窗口打开用户端，
 * 这是管理端唯一需要知道用户端地址的地方。
 */
export const USER_CONSOLE_URL = normalizeBase(
  import.meta.env.VITE_USER_BASE_URL as string | undefined,
  'http://localhost:3002',
)
