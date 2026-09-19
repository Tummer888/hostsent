#!/usr/bin/env python3
"""依据路由注册生成 backend/docs/openapi.yaml。

背景（doc104 §3.3 F24）：此前 openapi.yaml 是手写的，随代码演进严重过期
—— 16 处已退役的 /distribution/* 路径还在，管理端前缀写成 /api/v1/users
而非 /api/v1/admin/users，安全 / 实名 / 等级等新模块整体缺失。

手工维护一份 440+ 路径的文档必然再次过期，所以改成生成式：

    internal/server/*.go         唯一事实来源（路由注册）
    docs/openapi.template.yaml   手工维护 info / servers / components
    scripts/openapi_overrides.json  逐条补充描述、参数、请求体、响应
    docs/openapi.yaml            生成产物，勿手改

用法：python3 scripts/gen_openapi.py

生成内容的规则刻意机械：
  - tags 由 TAG_RULES 推导（声明与使用同源，不会漂移）；
  - summary 由「路径名词 + handler 动词」拼装，名词取 PATH_NOUN_RULES 的
    最长匹配前缀（比按 tag 取词精确一级），拼不通的由 overrides 覆盖；
  - security 由中间件链判定（app.perm/adminAuth → BearerAuth，
    open.Gateway/RequireScope → OpenSignature，无中间件 → 不写 security）；
  - 路径参数、分页参数、权限码同理自动补齐。

自检三道，任一不通过即非零退出 —— 缺文档比错文档好，静默漏掉更糟：
  1. 每条注册路由都必须能归到某个 tag；
  2. 每条注册路由都必须能推出 summary；
  3. 生成结果里所有 $ref 都必须能在 components 里找到。
"""

import json
import re
import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parent.parent
TEMPLATE = ROOT / "docs/openapi.template.yaml"
OUTPUT = ROOT / "docs/openapi.yaml"
OVERRIDES = ROOT / "scripts/openapi_overrides.json"
SOURCES = [
    ROOT / "internal/server/router.go",
    ROOT / "internal/server/assembly_oauth.go",
]

VERBS = ("GET", "POST", "PUT", "PATCH", "DELETE")
METHOD_ORDER = ["get", "post", "put", "patch", "delete"]

# 分组变量的初始前缀。gin 的路由树挂在哪个 group 上决定了最终路径，
# 这里把 r / v1 / uc 等根分组手工登记，其余 group 由 Group("...") 调用推导。
ROOT_GROUPS = {
    "r": "",
    "v1": "/api/v1/admin",
    "admin": "/api/v1/admin",
    "uc": "/api/v1/uc",
    "openV1": "/open/v1",
    # 兼容分组：为 frontend-user / frontend-site 的历史 baseURL 保留的别名。
    "ucAuthCompat": "/api/v1/auth",
    "ucMenuCompat": "/api/v1/menus",
    "publicSite": "/api/v1/public",
    "publicCaptcha": "/api/v1/public",
    "paymentNotify": "/api/v1/payment/notify",
}

# 路径前缀 → tag，最长前缀优先。每条注册路由都必须命中一条，否则生成失败。
TAG_RULES = [
    ("/health", "系统"),
    ("/ready", "系统"),
    ("/metrics", "系统"),
    ("/api/v1/admin/auth/admins", "管理端-认证"),
    ("/api/v1/admin/auth", "管理端-认证"),
    ("/api/v1/admin/staff", "管理端-员工组织"),
    ("/api/v1/admin/departments", "管理端-员工组织"),
    ("/api/v1/admin/users", "管理端-用户"),
    ("/api/v1/admin/user-groups", "管理端-RBAC"),
    ("/api/v1/admin/user-levels", "管理端-RBAC"),
    ("/api/v1/admin/roles", "管理端-RBAC"),
    ("/api/v1/admin/permissions", "管理端-RBAC"),
    ("/api/v1/admin/menus", "管理端-RBAC"),
    ("/api/v1/admin/verifications", "管理端-实名认证"),
    ("/api/v1/admin/oauth", "管理端-第三方登录"),
    ("/api/v1/admin/security", "管理端-安全"),
    ("/api/v1/admin/audit-logs", "管理端-安全"),
    ("/api/v1/admin/system", "管理端-系统设置"),
    ("/api/v1/admin/captcha", "管理端-验证码安全"),
    ("/api/v1/admin/logs", "管理端-日志中心"),
    ("/api/v1/admin/notification-templates", "管理端-通知中心"),
    ("/api/v1/admin/notification", "管理端-通知中心"),
    ("/api/v1/admin/notifications", "管理端-通知中心"),
    ("/api/v1/admin/announcements", "管理端-通知中心"),
    ("/api/v1/admin/content", "管理端-内容中心"),
    ("/api/v1/admin/payment", "管理端-支付中心"),
    ("/api/v1/admin/finance", "管理端-财务"),
    ("/api/v1/admin/refunds", "管理端-财务"),
    ("/api/v1/admin/renewals", "管理端-财务"),
    ("/api/v1/admin/orders", "管理端-订单"),
    ("/api/v1/admin/instances", "管理端-实例"),
    ("/api/v1/admin/lifecycle", "管理端-实例"),
    ("/api/v1/admin/resource", "管理端-资源"),
    ("/api/v1/admin/product", "管理端-商品"),
    ("/api/v1/admin/points", "管理端-积分"),
    ("/api/v1/admin/referral", "管理端-推广返现"),
    ("/api/v1/admin/sales", "管理端-销售"),
    ("/api/v1/admin/ticket-categories", "管理端-工单"),
    ("/api/v1/admin/tickets", "管理端-工单"),
    ("/api/v1/uc/auth", "用户端-认证"),
    ("/api/v1/uc/verification", "用户端-实名认证"),
    ("/api/v1/uc/oauth", "用户端-第三方登录"),
    ("/api/v1/uc/finance", "用户端-财务"),
    ("/api/v1/uc/payment", "用户端-财务"),
    ("/api/v1/uc/orders", "用户端-订单"),
    ("/api/v1/uc/products", "用户端-商品"),
    ("/api/v1/uc/instances", "用户端-实例"),
    ("/api/v1/uc/renewals", "用户端-实例"),
    ("/api/v1/uc/members", "用户端-子账号"),
    ("/api/v1/uc/notification-preferences", "用户端-消息中心"),
    ("/api/v1/uc/notifications", "用户端-消息中心"),
    ("/api/v1/uc/announcements", "用户端-消息中心"),
    ("/api/v1/uc/menus", "用户端-认证"),
    ("/api/v1/uc/points", "用户端-积分"),
    ("/api/v1/uc/referral", "用户端-推广返现"),
    ("/api/v1/uc/security", "用户端-验证码安全"),
    ("/api/v1/uc/support", "用户端-工单"),
    ("/api/v1/auth", "用户端-认证（兼容别名）"),
    ("/api/v1/menus", "用户端-认证（兼容别名）"),
    ("/api/v1/public", "前台公开"),
    ("/api/v1/payment/notify", "支付回调"),
    ("/open/v1/audit", "开放平台"),
    ("/open/v1", "开放平台"),
]

# tag 的展示描述。
TAG_DESC = {
    "系统": "健康检查、就绪探针与运行指标",
    "管理端-认证": "管理员登录、自助与旧路径别名 /auth/admins",
    "管理端-员工组织": "员工账号与组织部门",
    "管理端-用户": "用户列表 / 详情 / 注销留存期 / 导出",
    "管理端-RBAC": "角色、权限、菜单、用户组、用户等级",
    "管理端-实名认证": "实名申请整单审核、核验服务商与策略配置",
    "管理端-第三方登录": "微信 / QQ / 支付宝渠道配置与绑定关系",
    "管理端-安全": "登录日志、审计日志、风控事件、黑名单、会话",
    "管理端-系统设置": "系统配置项读写",
    "管理端-验证码安全": "验证码渠道、场景策略与统计",
    "管理端-日志中心": "统一日志查询、导出与留存清理",
    "管理端-通知中心": "通知渠道、模板、群发与投递记录",
    "管理端-内容中心": "文章、分类与友情链接",
    "管理端-支付中心": "支付渠道、支付单、对账与出款",
    "管理端-财务": "账单、发票、退款、流水与续费",
    "管理端-订单": "订单查询与状态流转",
    "管理端-实例": "实例运维与生命周期策略",
    "管理端-资源": "上游服务商、资源池、同步框架与实例",
    "管理端-商品": "商品、规格、价格与促销",
    "管理端-积分": "积分账户、规则与流水",
    "管理端-推广返现": "邀请关系、返现与提现",
    "管理端-销售": "客户归属、佣金、业绩与提现",
    "管理端-工单": "工单、回复审核与附件",
    "用户端-认证": "用户注册登录、资料与密码自助",
    "用户端-实名认证": "实名状态、提交、材料与三方核验回调",
    "用户端-第三方登录": "授权、免登录回调、票据换令牌与绑定管理",
    "用户端-财务": "余额、账单、发票与充值提现",
    "用户端-订单": "下单、支付与取消",
    "用户端-商品": "商品浏览",
    "用户端-实例": "实例自助与续费",
    "用户端-子账号": "子账号成员与权限",
    "用户端-消息中心": "站内通知与通知偏好",
    "用户端-积分": "我的积分与流水",
    "用户端-推广返现": "邀请资料、返现与提现",
    "用户端-验证码安全": "验证码设置与关键操作验证",
    "用户端-工单": "工单提交与回复",
    "用户端-认证（兼容别名）": "为历史 baseURL=/api/v1 保留的 /auth、/menus 别名",
    "前台公开": "门户站点公开数据（免登录）",
    "支付回调": "第三方支付异步通知（免登录，验签后驱动）",
    "开放平台": "对外 OpenAPI（签名 + 能力位）",
}

# 路径前缀 → 资源名词，最长前缀优先，未命中时回落到 TAG_NOUN。
# 比按 tag 取词精确一级：同一 tag 下的 /finance/bills 与 /finance/invoices
# 名词不同，只按 tag 取词会拼出「开具财务记录」这种读不通的摘要。
PATH_NOUN_RULES = [
    ("/api/v1/admin/auth/admins", "管理员账号"),
    ("/api/v1/admin/auth/login/verify-otp", "登录二次验证"),
    ("/api/v1/admin/auth/login", "管理员登录"),
    ("/api/v1/admin/auth/me", "当前管理员"),
    ("/api/v1/admin/auth/change-password", "管理员密码"),
    ("/api/v1/admin/auth", "管理员登录"),
    ("/api/v1/admin/staff", "员工"),
    ("/api/v1/admin/departments", "部门"),
    ("/api/v1/admin/users/export", "用户 CSV"),
    ("/api/v1/admin/users/stats", "用户统计"),
    ("/api/v1/admin/users/region-stats", "用户地域分布"),
    ("/api/v1/admin/users/purge", "留存期清理"),
    ("/api/v1/admin/users/batch-delete", "用户批量注销"),
    ("/api/v1/admin/users/batch-restore", "用户批量恢复"),
    ("/api/v1/admin/users/{id}/deletion-check", "注销前置校验"),
    ("/api/v1/admin/users/{id}/detail-aggregate", "用户详情聚合"),
    ("/api/v1/admin/users/{id}/impersonate", "代登录"),
    ("/api/v1/admin/users/{id}/recharge", "余额充值"),
    ("/api/v1/admin/users/{id}/orders", "代客订单"),
    ("/api/v1/admin/users/{id}/members", "子账号成员"),
    ("/api/v1/admin/users/{id}/restore", "用户恢复"),
    ("/api/v1/admin/users/{id}/roles", "用户角色"),
    ("/api/v1/admin/users/{id}/status", "用户状态"),
    ("/api/v1/admin/users/{id}/reset-password", "用户密码"),
    ("/api/v1/admin/users/{id}", "用户"),
    ("/api/v1/admin/users", "用户"),
    ("/api/v1/admin/user-groups", "用户组"),
    ("/api/v1/admin/user-levels", "用户等级"),
    ("/api/v1/admin/roles", "角色"),
    ("/api/v1/admin/permissions", "权限"),
    ("/api/v1/admin/menus", "菜单"),
    ("/api/v1/admin/verifications/configs", "实名策略配置"),
    ("/api/v1/admin/verifications/provider-types", "核验服务商类型"),
    ("/api/v1/admin/verifications/providers", "核验服务商"),
    ("/api/v1/admin/verifications/documents", "实名材料"),
    ("/api/v1/admin/verifications/pending", "待审实名申请"),
    ("/api/v1/admin/verifications/approved", "已通过实名申请"),
    ("/api/v1/admin/verifications/rejected", "已驳回实名申请"),
    ("/api/v1/admin/verifications", "实名申请"),
    ("/api/v1/admin/oauth/provider-types", "第三方渠道类型"),
    ("/api/v1/admin/oauth/providers", "第三方渠道"),
    ("/api/v1/admin/oauth/bindings", "第三方绑定"),
    ("/api/v1/admin/security/login-logs", "登录日志"),
    ("/api/v1/admin/security/audit-logs", "安全审计日志"),
    ("/api/v1/admin/security/risk-events", "风控事件"),
    ("/api/v1/admin/security/blacklists", "黑名单"),
    ("/api/v1/admin/security/sessions", "在线会话"),
    ("/api/v1/admin/audit-logs", "管理端操作审计"),
    ("/api/v1/admin/system/configs", "系统配置"),
    ("/api/v1/admin/captcha/policies", "验证码策略"),
    ("/api/v1/admin/captcha/providers", "验证码渠道"),
    ("/api/v1/admin/captcha/stats", "验证码统计"),
    ("/api/v1/admin/logs/cleanup-jobs", "日志清理任务"),
    ("/api/v1/admin/logs/cleanup", "日志清理"),
    ("/api/v1/admin/logs/export-files", "日志导出文件"),
    ("/api/v1/admin/logs/policies", "日志留存策略"),
    ("/api/v1/admin/logs/query", "日志"),
    ("/api/v1/admin/logs/stats", "日志统计"),
    ("/api/v1/admin/logs/catalog", "日志来源目录"),
    ("/api/v1/admin/logs", "日志"),
    ("/api/v1/admin/notification-templates", "通知模板"),
    ("/api/v1/admin/notification/broadcast", "站内群发"),
    ("/api/v1/admin/notification/channels", "通知渠道"),
    ("/api/v1/admin/notification/channel-types", "通知渠道类型"),
    ("/api/v1/admin/notification/deliveries", "通知投递记录"),
    ("/api/v1/admin/notification/sms-templates", "短信模板"),
    ("/api/v1/admin/notification/template-vars", "模板变量"),
    ("/api/v1/admin/notification/test-send", "测试消息"),
    ("/api/v1/admin/notification", "通知"),
    ("/api/v1/admin/notifications", "站内通知"),
    ("/api/v1/admin/announcements", "公告"),
    ("/api/v1/admin/content/articles", "文章"),
    ("/api/v1/admin/content/categories", "内容分类"),
    ("/api/v1/admin/content/links", "友情链接"),
    ("/api/v1/admin/payment/callbacks", "支付回调记录"),
    ("/api/v1/admin/payment/channels", "支付渠道"),
    ("/api/v1/admin/payment/channel-types", "支付渠道类型"),
    ("/api/v1/admin/payment/orders", "支付单"),
    ("/api/v1/admin/payment/payouts", "出款单"),
    ("/api/v1/admin/payment/recon", "支付对账"),
    ("/api/v1/admin/payment/refunds", "退款单"),
    ("/api/v1/admin/payment/methods", "支付方式"),
    ("/api/v1/admin/finance/bills", "账单"),
    ("/api/v1/admin/finance/invoices", "发票"),
    ("/api/v1/admin/finance/recharges", "充值单"),
    ("/api/v1/admin/finance/transactions", "资金流水"),
    ("/api/v1/admin/finance/wallets", "钱包"),
    ("/api/v1/admin/finance/withdrawals", "提现单"),
    ("/api/v1/admin/refunds", "退款单"),
    ("/api/v1/admin/renewals", "续费记录"),
    ("/api/v1/admin/orders", "订单"),
    ("/api/v1/admin/instances", "实例"),
    ("/api/v1/admin/lifecycle", "生命周期策略"),
    ("/api/v1/admin/resource/providers", "上游服务商"),
    ("/api/v1/admin/resource/pools", "资源池"),
    ("/api/v1/admin/resource/products", "上游商品"),
    ("/api/v1/admin/resource/sync/schedules", "同步调度"),
    ("/api/v1/admin/resource/sync/price-changes", "同步价格变动"),
    ("/api/v1/admin/resource/sync/diffs", "同步差异"),
    ("/api/v1/admin/resource/sync/tasks", "同步任务"),
    ("/api/v1/admin/resource/sync/scopes", "同步范围"),
    ("/api/v1/admin/resource/sync/logs", "同步日志"),
    ("/api/v1/admin/resource/sync", "同步任务"),
    ("/api/v1/admin/resource/task-queue", "异步任务队列"),
    ("/api/v1/admin/resource/reconcile", "资源对账"),
    ("/api/v1/admin/resource/instances", "上游实例"),
    ("/api/v1/admin/product/categories", "商品分类"),
    ("/api/v1/admin/product/discount-policies", "折扣策略"),
    ("/api/v1/admin/product/prices", "商品价格"),
    ("/api/v1/admin/product/pricing", "定价方案"),
    ("/api/v1/admin/product/products", "商品"),
    ("/api/v1/admin/product/promotion/coupons", "优惠券"),
    ("/api/v1/admin/product/promotion/coupon-grants", "优惠券发放"),
    ("/api/v1/admin/product/promotion/promotions", "促销活动"),
    ("/api/v1/admin/product/spec/atoms", "规格原子"),
    ("/api/v1/admin/product/spec/bindings", "规格绑定"),
    ("/api/v1/admin/product/spec/external-specs", "外部规格"),
    ("/api/v1/admin/product/spec/mappings", "规格映射"),
    ("/api/v1/admin/product/spec/templates", "规格模板"),
    ("/api/v1/admin/points/accounts", "积分账户"),
    ("/api/v1/admin/points/rules", "积分规则"),
    ("/api/v1/admin/points/transactions", "积分流水"),
    ("/api/v1/admin/points/overview", "积分总览"),
    ("/api/v1/admin/referral/cashbacks", "返现记录"),
    ("/api/v1/admin/referral/invitees", "被邀请人"),
    ("/api/v1/admin/referral/withdrawals", "返现提现单"),
    ("/api/v1/admin/sales/commissions", "销售佣金"),
    ("/api/v1/admin/sales/customers", "销售客户"),
    ("/api/v1/admin/sales/performance", "销售业绩"),
    ("/api/v1/admin/sales/sales-candidates", "候选销售"),
    ("/api/v1/admin/sales/withdrawals", "销售提现单"),
    ("/api/v1/admin/tickets/attachments", "工单附件"),
    ("/api/v1/admin/tickets/replies", "工单回复"),
    ("/api/v1/admin/tickets/reviews", "工单评价"),
    ("/api/v1/admin/tickets/stats", "工单统计"),
    ("/api/v1/admin/tickets", "工单"),
    ("/api/v1/admin/ticket-categories", "工单分类"),
    ("/api/v1/uc/auth/login/verify-otp", "登录二次验证"),
    ("/api/v1/uc/auth/login", "登录"),
    ("/api/v1/uc/auth/register", "注册"),
    ("/api/v1/uc/auth/forgot-password", "忘记密码"),
    ("/api/v1/uc/auth/reset-password", "重置密码"),
    ("/api/v1/uc/auth/logout", "登出"),
    ("/api/v1/uc/auth/userinfo", "用户信息"),
    ("/api/v1/uc/auth/profile", "个人资料"),
    ("/api/v1/uc/auth/password", "登录密码"),
    ("/api/v1/uc/verification/applications", "实名申请"),
    ("/api/v1/uc/verification/documents", "实名材料"),
    ("/api/v1/uc/verification", "实名申请"),
    ("/api/v1/uc/oauth/bindings", "第三方绑定"),
    ("/api/v1/uc/oauth/exchange", "登录票据"),
    ("/api/v1/uc/oauth/providers", "可用登录渠道"),
    ("/api/v1/uc/oauth", "第三方登录"),
    ("/api/v1/uc/finance/invoices", "发票"),
    ("/api/v1/uc/finance/recharges", "充值单"),
    ("/api/v1/uc/finance/transactions", "资金流水"),
    ("/api/v1/uc/finance/bills", "账单"),
    ("/api/v1/uc/finance/balance", "余额"),
    ("/api/v1/uc/finance/recharge", "充值"),
    ("/api/v1/uc/payment/accounts", "支付账户"),
    ("/api/v1/uc/payment/bills", "账单支付"),
    ("/api/v1/uc/payment/orders", "支付单"),
    ("/api/v1/uc/payment/preferences", "支付偏好"),
    ("/api/v1/uc/payment/methods", "支付方式"),
    ("/api/v1/uc/payment/recharge", "充值"),
    ("/api/v1/uc/payment/withdrawals", "提现申请"),
    ("/api/v1/uc/orders/quote", "订单询价"),
    ("/api/v1/uc/orders", "订单"),
    ("/api/v1/uc/products", "商品"),
    ("/api/v1/uc/instances/renewals", "续费管理"),
    ("/api/v1/uc/instances", "实例"),
    ("/api/v1/uc/renewals", "续费记录"),
    ("/api/v1/uc/members", "子账号成员"),
    ("/api/v1/uc/notifications", "站内通知"),
    ("/api/v1/uc/notification-preferences", "通知偏好"),
    ("/api/v1/uc/announcements", "公告"),
    ("/api/v1/uc/menus", "用户端菜单"),
    ("/api/v1/uc/points", "积分"),
    ("/api/v1/uc/referral/cashbacks", "返现记录"),
    ("/api/v1/uc/referral/invitees", "被邀请人"),
    ("/api/v1/uc/referral/profile", "邀请资料"),
    ("/api/v1/uc/referral/transfer", "返现划转"),
    ("/api/v1/uc/referral/withdrawals", "提现申请"),
    ("/api/v1/uc/security/settings", "验证码设置"),
    ("/api/v1/uc/security/verification", "关键操作验证"),
    ("/api/v1/uc/support/attachments", "工单附件"),
    ("/api/v1/uc/support/tickets", "工单"),
    ("/api/v1/uc/support/ticket-categories", "工单分类"),
    ("/api/v1/public/announcements", "站点公告"),
    ("/api/v1/public/article-categories", "文章分类"),
    ("/api/v1/public/article-singletons", "单页文章"),
    ("/api/v1/public/articles", "文章"),
    ("/api/v1/public/auth-config", "登录方式配置"),
    ("/api/v1/public/captcha", "图形验证码"),
    ("/api/v1/public/friendly-links", "友情链接"),
    ("/api/v1/public/site-content", "站点内容"),
    ("/api/v1/public/verify-code", "验证码"),
    ("/api/v1/payment/notify", "支付回调"),
    ("/api/v1/auth", "账号"),
    ("/api/v1/menus", "用户端菜单"),
    ("/open/v1/spec-atoms", "规格原子目录"),
    ("/open/v1/products", "商品目录"),
    ("/open/v1/regions", "地域目录"),
    ("/open/v1/images", "镜像目录"),
    ("/open/v1/quote", "询价"),
    ("/open/v1/orders", "代客订单"),
    ("/open/v1/instances", "实例"),
    ("/open/v1/audit/orders", "订单审计"),
    ("/open/v1/audit/requests", "请求审计"),
    ("/open/v1/ping", "链路"),
]

# 未命中 PATH_NOUN_RULES 时按 tag 取词。
TAG_NOUN = {
    "系统": "服务状态",
    "管理端-认证": "管理员",
    "管理端-员工组织": "员工",
    "管理端-用户": "用户",
    "管理端-RBAC": "权限配置",
    "管理端-实名认证": "实名申请",
    "管理端-第三方登录": "第三方渠道",
    "管理端-安全": "安全记录",
    "管理端-系统设置": "系统配置",
    "管理端-验证码安全": "验证码配置",
    "管理端-日志中心": "日志",
    "管理端-通知中心": "通知",
    "管理端-内容中心": "内容",
    "管理端-支付中心": "支付渠道",
    "管理端-财务": "财务记录",
    "管理端-订单": "订单",
    "管理端-实例": "实例",
    "管理端-资源": "上游资源",
    "管理端-商品": "商品",
    "管理端-积分": "积分",
    "管理端-推广返现": "推广返现",
    "管理端-销售": "销售数据",
    "管理端-工单": "工单",
    "用户端-认证": "账号",
    "用户端-认证（兼容别名）": "账号",
    "用户端-实名认证": "实名申请",
    "用户端-第三方登录": "第三方登录",
    "用户端-财务": "账单",
    "用户端-订单": "订单",
    "用户端-商品": "商品",
    "用户端-实例": "实例",
    "用户端-子账号": "子账号",
    "用户端-消息中心": "消息",
    "用户端-积分": "积分",
    "用户端-推广返现": "推广返现",
    "用户端-验证码安全": "验证码设置",
    "用户端-工单": "工单",
    "前台公开": "前台数据",
    "支付回调": "支付回调",
    "开放平台": "开放平台资源",
}

# 三个探针是匿名闭包（没有 handler 名），摘要单独给。
PATH_SUMMARIES = {
    "/health": "健康检查",
    "/ready": "就绪检查",
    "/metrics": "运行指标快照",
}

# handler 名 → (动词, 语序)。语序 "post" 表示名词在前（「用户列表」），
# "pre" 表示动词在前（「新建用户」）。顺序敏感：更长的前缀要排在更前面。
HANDLER_ACTIONS = [
    # (handler 前缀, 动作短语, 语序)。语序 "post" = 名词在前（「用户列表」），
    # "pre" = 动词在前（「新建用户」），"none" = 动作短语即完整摘要。
    ("VerifyLoginOTP", "登录二次验证", "post"),
    ("ChangePassword", "修改", "pre"),
    ("ResetPassword", "重置", "pre"),
    ("UpdateUserStatus", "更新", "pre"),
    ("UpdateStatus", "更新状态", "post"),
    ("GetRegionStats", "统计", "post"),
    ("GetStats", "统计", "post"),
    ("CheckDeletion", "注销前置校验", "none"),
    ("GetAggregate", "详情聚合", "post"),
    ("ProviderCheck", "三方核验", "post"),
    ("ProviderTypes", "元数据", "post"),
    ("TestProvider", "连通性测试", "post"),
    ("ListProviders", "列表", "post"),
    ("UpdateProvider", "更新", "pre"),
    ("DeleteProvider", "删除", "pre"),
    ("UpsertProvider", "新建或更新", "pre"),
    ("ListPolicies", "列表", "post"),
    ("UpdatePolicy", "更新", "pre"),
    ("ListSchedules", "列表", "post"),
    ("UpdateSchedule", "更新", "pre"),
    ("ListPriceChanges", "列表", "post"),
    ("HandlePriceChanges", "处理", "pre"),
    ("DiffSummary", "汇总", "post"),
    ("ListDiffs", "列表", "post"),
    ("ScopeMeta", "元数据", "post"),
    ("ListTasks", "列表", "post"),
    ("ListLogs", "列表", "post"),
    ("ListTypes", "元数据", "post"),
    ("ListInstances", "列表", "post"),
    ("GetInstance", "详情", "post"),
    ("ListMembers", "列表", "post"),
    ("ListRoles", "列表", "post"),
    ("CreateRole", "新建", "pre"),
    ("GetRole", "详情", "post"),
    ("UpdateRole", "更新", "pre"),
    ("DeleteRole", "删除", "pre"),
    ("SetRoles", "分配角色", "post"),
    ("AssignRoles", "分配角色", "post"),
    ("AssignPermissions", "分配", "pre"),
    ("GetRolePermissions", "详情", "post"),
    ("ListUsers", "列表", "post"),
    ("CreateUser", "新建", "pre"),
    ("GetUser", "详情", "post"),
    ("UpdateUser", "更新", "pre"),
    ("DeleteUser", "注销", "pre"),
    ("RestoreUser", "恢复", "pre"),
    ("ExportUsers", "导出", "pre"),
    ("Impersonate", "代登录", "none"),
    ("CreateOrder", "代客下单", "none"),
    ("Recharge", "充值", "pre"),
    ("PurgeUsers", "留存期清理", "none"),
    ("BatchDeleteUsers", "批量注销", "none"),
    ("BatchRestoreUsers", "批量恢复", "none"),
    ("ExportLoginLogs", "导出", "pre"),
    ("ExportAuditLogs", "导出", "pre"),
    ("ListLoginLogs", "列表", "post"),
    ("GetLoginLog", "详情", "post"),
    ("ListAuditLogs", "列表", "post"),
    ("GetAuditLog", "详情", "post"),
    ("ListRiskEvents", "列表", "post"),
    ("GetRiskEvent", "详情", "post"),
    ("IgnoreRiskEvent", "忽略", "pre"),
    ("HandleRiskEvent", "处置", "pre"),
    ("CreateBlacklistFromRisk", "从风控事件拉黑", "none"),
    ("RevokeSessionsFromRisk", "失效该事件相关会话", "none"),
    ("ListBlacklists", "列表", "post"),
    ("CreateBlacklist", "新建", "pre"),
    ("GetBlacklist", "详情", "post"),
    ("UpdateBlacklistStatus", "更新状态", "post"),
    ("UpdateBlacklist", "更新", "pre"),
    ("ReleaseBlacklist", "解除", "pre"),
    ("ListBlacklistHits", "命中记录", "post"),
    ("ListSessions", "列表", "post"),
    ("GetSession", "详情", "post"),
    ("RevokeSession", "失效", "pre"),
    ("BatchRevokeSessions", "批量失效", "pre"),
    ("RevokeUserAllSessions", "失效该用户全部会话", "none"),
    ("ListBindings", "列表", "post"),
    ("UploadDocument", "上传", "pre"),
    ("DownloadDocument", "下载", "pre"),
    ("DownloadAttachment", "下载", "pre"),
    ("UploadAttachment", "上传", "pre"),
    ("UpdateTemplate", "更新", "pre"),
    ("ListTemplates", "列表", "post"),
    ("MarkPaid", "标记已付", "none"),
    ("MarkFailed", "标记失败", "none"),
    ("UpdatePrice", "更新价格", "post"),
    ("Reconcile", "对账", "pre"),
    ("RunCleanup", "执行", "pre"),
    ("DownloadExportFile", "下载", "pre"),
    ("BatchCloneFromUpstream", "批量从上游复制", "pre"),
    ("UpsertExternalSpec", "新建或更新", "pre"),
    ("UpsertTargets", "新建或更新", "pre"),
    ("UpsertConfig", "新建或更新", "pre"),
    ("UpsertBinding", "新建或更新", "pre"),
    ("BatchUpsert", "批量新建或更新", "pre"),
    ("BatchRetry", "批量重试", "pre"),
    ("InvoiceDownload", "下载", "pre"),
    ("InvoiceEmail", "邮件发送", "pre"),
    ("ApplyInvoice", "申请", "pre"),
    ("ApplyWithdrawal", "发起", "pre"),
    ("PayWithdrawal", "打款", "pre"),
    ("PayBill", "支付", "pre"),
    ("Pay", "支付", "pre"),
    ("SetDefaultAccount", "设为默认", "pre"),
    ("SavePreferences", "保存", "pre"),
    ("SetPermissions", "设置权限", "post"),
    ("SetRemark", "更新", "pre"),
    ("SetFeatured", "设为推荐", "pre"),
    ("SaveConfigOptions", "保存", "pre"),
    ("ToggleAutoRenew", "切换自动续费", "none"),
    ("SendMailTest", "测试邮件发送", "none"),
    ("SendVerification", "下发验证码", "none"),
    ("SendCode", "下发验证码", "none"),
    ("Send", "发送", "pre"),
    ("ForgotPassword", "忘记密码", "none"),
    ("AdminUnreadCount", "管理端未读数", "none"),
    ("UserUnreadCount", "未读数", "post"),
    ("UserReadAll", "全部标记已读", "post"),
    ("UserAnnouncements", "我的公告", "none"),
    ("UserGetPrefs", "偏好", "post"),
    ("UserUpdatePrefs", "更新", "pre"),
    ("UserDetail", "详情", "post"),
    ("UserList", "列表", "post"),
    ("UserInfo", "用户信息", "none"),
    ("AuthConfig", "登录方式配置", "none"),
    ("SiteContent", "站点内容", "none"),
    ("FriendlyLinks", "友情链接", "none"),
    ("ArticleCategories", "文章分类", "none"),
    ("ArticleDetail", "文章详情", "none"),
    ("SingletonArticle", "单页文章", "none"),
    ("AnnouncementDetail", "公告详情", "none"),
    ("ImageChallenge", "图形验证码", "none"),
    ("OpenVerification", "要求", "post"),
    ("ProviderCallback", "回调", "post"),
    ("Providers", "列表", "post"),
    ("MyBindings", "我的绑定", "none"),
    ("MyInvoices", "我的发票", "none"),
    ("Invoices", "列表", "post"),
    ("Balance", "余额", "post"),
    ("Bills", "列表", "post"),
    ("Records", "列表", "post"),
    ("Transactions", "列表", "post"),
    ("Account", "账户", "post"),
    ("Accounts", "列表", "post"),
    ("Withdrawals", "列表", "post"),
    ("Customers", "列表", "post"),
    ("Commissions", "列表", "post"),
    ("Performance", "业绩", "post"),
    ("Candidates", "列表", "post"),
    ("Relations", "归属关系", "post"),
    ("Notifications", "列表", "post"),
    ("Announcements", "列表", "post"),
    ("Articles", "列表", "post"),
    ("Categories", "列表", "post"),
    ("Links", "列表", "post"),
    ("Products", "列表", "post"),
    ("Regions", "列表", "post"),
    ("Images", "列表", "post"),
    ("Points", "总览", "post"),
    ("Bindings", "列表", "post"),
    ("Sessions", "列表", "post"),
    ("Events", "列表", "post"),
    ("Blacklists", "列表", "post"),
    ("Documents", "列表", "post"),
    ("Applications", "列表", "post"),
    ("Settings", "详情", "post"),
    ("Preferences", "偏好", "post"),
    ("Menus", "树", "post"),
    ("Rules", "列表", "post"),
    ("Summary", "汇总", "post"),
    ("Unassigned", "未分配列表", "none"),
    ("Cashbacks", "列表", "post"),
    ("Invitees", "列表", "post"),
    ("Operations", "操作记录", "post"),
    ("Related", "关联资源", "post"),
    ("Options", "读取", "pre"),
    ("Order", "详情", "post"),
    ("Notify", "回调通知", "none"),
    ("Requirement", "要求", "post"),
    ("Overview", "总览", "post"),
    ("Ranking", "排行", "post"),
    ("Targets", "目标", "post"),
    ("Stats", "统计", "post"),
    ("Methods", "列表", "post"),
    ("Me", "当前身份", "post"),
    ("Profile", "资料", "post"),
    ("Status", "状态", "post"),
    ("Tree", "树", "post"),
    ("Query", "查询", "pre"),
    ("Catalog", "目录", "post"),
    ("Deliveries", "投递记录", "post"),
    ("Destroy", "销毁", "pre"),
    ("Activate", "激活", "pre"),
    ("Release", "释放", "pre"),
    ("Assign", "分配", "pre"),
    ("Withdraw", "提现", "pre"),
    ("Login", "登录", "pre"),
    ("Logout", "登出", "pre"),
    ("Register", "注册", "pre"),
    ("Submit", "提交", "pre"),
    ("Approve", "通过", "pre"),
    ("Reject", "驳回", "pre"),
    ("Revoke", "失效", "pre"),
    ("Resign", "离职", "pre"),
    ("Quote", "询价", "pre"),
    ("Ping", "链路自检", "none"),
    ("Publish", "发布", "pre"),
    ("Unpublish", "下架", "pre"),
    ("Offline", "下线", "pre"),
    ("Broadcast", "群发", "pre"),
    ("Preview", "预览", "pre"),
    ("Resend", "重发", "pre"),
    ("Retry", "重试", "pre"),
    ("Verify", "校验", "pre"),
    ("Validate", "校验", "pre"),
    ("Issue", "开具", "pre"),
    ("Confirm", "确认", "pre"),
    ("Claim", "认领", "pre"),
    ("Reply", "回复", "pre"),
    ("ReviewLogs", "审核轨迹", "post"),
    ("Review", "审核", "pre"),
    ("Transfer", "转派", "pre"),
    ("Resume", "恢复同步", "pre"),
    ("Unsuspend", "解除暂停", "pre"),
    ("Suspend", "暂停", "pre"),
    ("Power", "电源操作", "post"),
    ("Renew", "续费", "pre"),
    ("Resize", "变更规格", "pre"),
    ("Clone", "复制", "pre"),
    ("Sync", "同步", "pre"),
    ("Adjust", "调整", "pre"),
    ("Generate", "生成", "pre"),
    ("Cleanup", "清理", "pre"),
    ("Scan", "扫描", "pre"),
    ("Close", "关闭", "pre"),
    ("Cancel", "取消", "pre"),
    ("Bind", "绑定", "pre"),
    ("Unbind", "解绑", "pre"),
    ("Authorize", "发起授权", "none"),
    ("Callback", "回调", "post"),
    ("Exchange", "ticket 换令牌", "none"),
    ("Password", "密码", "post"),
    ("Detail", "详情", "post"),
    ("Get", "详情", "post"),
    ("List", "列表", "post"),
    ("Create", "新建", "pre"),
    ("Update", "更新", "pre"),
    ("Delete", "删除", "pre"),
    ("Export", "导出", "pre"),
    ("Test", "连通性测试", "post"),
    ("Save", "保存", "pre"),
    ("VNC", "远程控制台", "post"),
]

# 免登录路径 —— 这些路径上没有任何鉴权中间件，文档里也不该写 security。
PUBLIC_ALLOWLIST = {
    "/api/v1/admin/auth/login",
    "/api/v1/admin/auth/login/verify-otp",
    "/api/v1/public/auth-config",
    "/api/v1/public/captcha/image",
    "/api/v1/public/verify-code/send",
}

# 路径参数按整数渲染的名单；其余按字符串（provider / key / slug / kind 等）。
PATH_PARAM_INT = {"id", "userId", "user_id", "replyId", "specId"}
PATH_PARAM_STRING = {"provider", "key", "slug", "kind", "source", "scene", "group",
                     "channel_code", "payment_no", "provider_type"}

PAGE_HANDLERS = ("List", "Export")


def norm_path(path):
    """把 gin 的参数写法统一成 OpenAPI 的 {name}。"""
    # users.GET(":id") 里的 ":id" 紧跟在组路径后，需要补斜杠。
    path = re.sub(r"([^/])(?=[:*]\w)", r"\1/", path)
    path = re.sub(r"[*:](\w+)", r"{\1}", path)
    path = re.sub(r"/{2,}", "/", path)
    # 空字符串路由（group.PUT("")）拼出来会带尾斜杠，去掉它。
    return path.rstrip("/") or "/"


def strip_line_comment(line):
    """去掉行尾的 `// 注释`，但保留字符串里的 `//`（如 URL 字面量）。"""
    out, in_str, escaped, i = [], False, False, 0
    while i < len(line):
        ch = line[i]
        if escaped:
            out.append(ch)
            escaped = False
        elif ch == "\\":
            out.append(ch)
            escaped = True
        elif ch == '"':
            in_str = not in_str
            out.append(ch)
        elif not in_str and ch == "/" and i + 1 < len(line) and line[i + 1] == "/":
            break
        else:
            out.append(ch)
        i += 1
    return "".join(out)


def split_chain(rest):
    rest = rest.rstrip().rstrip(",").rstrip()
    return [c.strip() for c in rest.split(",") if c.strip()]


def parse_routes():
    """扫描 router 源码，返回 {path: {method: [middleware chain]}}。"""
    groups = dict(ROOT_GROUPS)
    routes = {}
    for src in SOURCES:
        lines = [strip_line_comment(l) for l in src.read_text().split("\n")]
        for line in lines:
            s = line.strip()
            if not s:
                continue
            m = re.match(r'(\w+)\s*:?=\s*(\w+)\.Group\("([^"]*)"\)', s)
            if m:
                var, parent, path = m.groups()
                groups[var] = groups.get(parent, "") + path
                continue
            m = re.match(r'(\w+)\.(%s)\("([^"]*)"(.*)' % "|".join(VERBS), s)
            if not m:
                continue
            var, verb, path, rest = m.groups()
            if var not in groups:
                continue
            routes.setdefault(norm_path(groups[var] + "/" + path), {})[verb.lower()] = split_chain(rest)
        # 直接挂在 r 上的：/health、/ready、/metrics 三个匿名闭包。
        for line in lines:
            m = re.match(r'\s*r\.(%s)\("([^"]*)"(.*)' % "|".join(VERBS), line)
            if m:
                verb, path, rest = m.groups()
                routes.setdefault(norm_path(path), {})[verb.lower()] = split_chain(rest)
    return routes


def longest_match(path, rules):
    best = None
    for prefix, value in rules:
        if path == prefix or path.startswith(prefix.rstrip("/") + "/") or path.startswith(prefix):
            if best is None or len(prefix) > len(best[0]):
                best = (prefix, value)
    return best[1] if best else None


def tag_for(path):
    return longest_match(path, TAG_RULES)


def noun_for(path, tag):
    return longest_match(path, PATH_NOUN_RULES) or TAG_NOUN.get(tag or "", "资源")


def handler_of(chain):
    if not chain:
        return ""
    last = chain[-1].rstrip(")")
    # 匿名闭包（/health 等）没有 handler 名。
    if last.endswith("{"):
        return ""
    return last.split(".")[-1]


def summarize(path, tag, handler):
    """拼出摘要：名词性动词放名词后（「用户列表」），及物动词放名词前（「新建用户」）。

    名词与动词互为子串时（路径名词是「积分账户」、动词也是「账户」）只取一个，
    否则会拼出「积分账户账户」这种叠词。
    """
    noun = noun_for(path, tag)
    for prefix, verb, order in HANDLER_ACTIONS:
        if not handler.startswith(prefix):
            continue
        if order == "none":
            return verb
        if verb in noun:
            return noun
        if noun in verb:
            return verb
        return f"{noun}{verb}" if order == "post" else f"{verb}{noun}"
    return None


def permission_of(chain):
    for c in chain:
        m = re.search(r'app\.perm\("([^"]+)"\)', c)
        if m:
            return m.group(1)
    for c in chain:
        m = re.search(r"app\.userPerm\((\w+)\)", c)
        if m:
            return m.group(1)
    return None


def auth_kind(path, chain):
    joined = " ".join(chain)
    if path in PUBLIC_ALLOWLIST:
        return "public"
    if "app.open.RequireScope(" in joined or "app.open.Gateway()" in joined:
        return "open"
    if "app.adminAuth()" in joined or "app.superOnly()" in joined or "app.perm(" in joined:
        return "admin"
    if ("middleware.UserAuth(" in joined or "app.userAuth()" in joined
            or "app.userPerm(" in joined or "app.rejectSub()" in joined):
        return "user"
    return "public"


def auto_parameters(path, method, handler):
    params = []
    for name in re.findall(r"\{(\w+)\}", path):
        if name in PATH_PARAM_INT:
            ptype = "integer"
        elif name in PATH_PARAM_STRING:
            ptype = "string"
        else:
            ptype = "string"
        params.append({"name": name, "in": "path", "required": True,
                       "schema": {"type": ptype}})
    if method == "get" and not params and handler.startswith(PAGE_HANDLERS):
        params.append({"$ref": "#/components/parameters/Page"})
        params.append({"$ref": "#/components/parameters/PageSize"})
    return params


def build_operation(path, method, chain, override):
    tag = override.get("tag") or tag_for(path)
    handler = handler_of(chain)
    op = {}
    if tag:
        op["tags"] = [tag]
    op["summary"] = (override.get("summary") or PATH_SUMMARIES.get(path)
                     or summarize(path, tag, handler) or f"{handler or method.upper()}")

    notes = []
    perm = permission_of(chain)
    if override.get("description"):
        notes.append(override["description"].strip())
    # overrides 的描述常常已经写明权限码，避免同一句出现两次。
    if perm and perm not in (override.get("description") or ""):
        notes.append(f"需要权限码 `{perm}`。")
    if "app.superOnly()" in " ".join(chain):
        notes.append("**仅超级管理员**可调用。")
    if "app.rejectSub()" in " ".join(chain):
        notes.append("子账号无独立资格，本接口对子账号硬拒绝。")
    if notes:
        op["description"] = "\n\n".join(notes)

    kind = override.get("security", auth_kind(path, chain))
    if kind in ("admin", "user"):
        op["security"] = [{"BearerAuth": []}]
    elif kind == "open":
        op["security"] = [{"OpenSignature": []}]

    params = override.get("parameters")
    if params is None:
        params = auto_parameters(path, method, handler)
    if params:
        op["parameters"] = params

    req = override.get("request")
    if req:
        op["requestBody"] = {
            "required": req.get("required", True),
            "content": {
                ctype: {"schema": ({"$ref": f"#/components/schemas/{schema}"}
                                   if isinstance(schema, str) else schema)}
                for ctype, schema in req["content"].items()
            },
        }

    responses = override.get("responses") or {"'200'": "success"}
    op["responses"] = {
        code: ({"description": spec} if isinstance(spec, str) else spec)
        for code, spec in responses.items()
    }
    return op


def collect_refs(node, found):
    if isinstance(node, dict):
        for k, v in node.items():
            if k == "$ref" and isinstance(v, str):
                found.add(v)
            else:
                collect_refs(v, found)
    elif isinstance(node, list):
        for item in node:
            collect_refs(item, found)


def main():
    overrides = json.loads(OVERRIDES.read_text())
    routes = parse_routes()

    unknown_tag = sorted(p for p in routes
                         if not overrides.get(p, {}).get("tag") and not tag_for(p))
    if unknown_tag:
        print("以下路由没有 tag 归属，请在 TAG_RULES 或 openapi_overrides.json 里补上：",
              file=sys.stderr)
        for p in unknown_tag:
            print("  " + p, file=sys.stderr)
        return 1

    no_summary = []
    for path, methods in routes.items():
        if path in PATH_SUMMARIES or overrides.get(path, {}).get("summary"):
            continue
        tag = overrides.get(path, {}).get("tag") or tag_for(path)
        for method, chain in methods.items():
            if not summarize(path, tag, handler_of(chain)):
                no_summary.append(f"{method.upper()} {path} ({handler_of(chain) or '匿名闭包'})")
    if no_summary:
        print("以下路由推不出 summary，请补 HANDLER_ACTIONS 或 openapi_overrides.json：",
              file=sys.stderr)
        for item in sorted(no_summary):
            print("  " + item, file=sys.stderr)
        return 1

    paths = {}
    for path in sorted(routes):
        override = overrides.get(path, {})
        methods = routes[path]
        ordered = sorted(methods, key=lambda m: METHOD_ORDER.index(m) if m in METHOD_ORDER else 99)
        paths[path] = {m: build_operation(path, m, methods[m], override) for m in ordered}

    template = TEMPLATE.read_text()
    for marker in ("# __TAGS__", "# __PATHS__"):
        if marker not in template:
            print(f"模板缺少 {marker} 占位符", file=sys.stderr)
            return 1

    head = yaml.safe_load(template.replace("# __TAGS__", "tags: []")
                                  .replace("# __PATHS__", "paths: {}"))
    # tags 也从 TAG_RULES 推导：声明与使用两处手写必然会漂移。
    declared = []
    for _, tag in TAG_RULES:
        if tag not in declared:
            declared.append(tag)
    head["tags"] = [{"name": t, "description": TAG_DESC.get(t, "")} for t in declared]
    head["paths"] = paths
    OUTPUT.write_text(
        "# 本文件由 scripts/gen_openapi.py 依据 internal/server 的路由注册生成。\n"
        "# 请勿手工编辑：改完路由后执行 `python3 scripts/gen_openapi.py`。\n"
        "# 需要补充描述 / 参数 / 响应时，改 scripts/openapi_overrides.json 或 docs/openapi.template.yaml。\n"
        + yaml.safe_dump(head, allow_unicode=True, sort_keys=False, width=1000)
    )

    # 自检：生成结果必须能重新解析，路径集合与代码注册一致。
    doc = yaml.safe_load(OUTPUT.read_text())
    missing = sorted(set(routes) - set(doc["paths"]))
    if missing:
        print("生成结果缺少路径：", missing, file=sys.stderr)
        return 1

    # 自检：所有 $ref 都能在 components 里找到 —— 断链的 $ref 比没有文档更糟。
    refs = set()
    collect_refs(doc["paths"], refs)
    schemas = set(doc["components"].get("schemas", {}))
    params = set(doc["components"].get("parameters", {}))
    dangling = []
    for ref in sorted(refs):
        kind, _, name = ref.rpartition("/")
        if kind.endswith("schemas") and name not in schemas:
            dangling.append(ref)
        elif kind.endswith("parameters") and name not in params:
            dangling.append(ref)
    if dangling:
        print("以下 $ref 在 components 里不存在：", file=sys.stderr)
        for ref in dangling:
            print("  " + ref, file=sys.stderr)
        return 1

    ops = sum(len(v) for v in doc["paths"].values())
    print(f"已生成 {OUTPUT.relative_to(ROOT)}：{len(doc['paths'])} 个路径 / {ops} 个操作 / "
          f"{len(refs)} 个 $ref 全部可解析")
    return 0


if __name__ == "__main__":
    sys.exit(main())
