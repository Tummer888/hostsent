#!/usr/bin/env python3
"""doc104 用户体系收口 HTTP 端到端验收（注销/恢复/回收站、实名整单审核、第三方登录）。

覆盖（docs/实施计划/104 §10.4）：
  A. 注销前置校验 → 注销 → 回收站可见 → 恢复 → 批量注销/批量恢复
     A1 注销后登录被拒，且提示是「用户名或密码错误」而不是「账号已被禁用」
        （后者等于向攻击者确认该账号存在）
     A2 注销后同 username 可重新注册（部分唯一索引只约束未注销行）
     A3 恢复时账号名被新用户占用 → 明确 409（不做静默改名）
     A4 留存期清理 dry_run 只预览、不写库
  B. 实名提交 → 管理端审核 → 用户端状态变化
     B1 证件号落密文、回显脱敏；改展示名不得被判定为已实名（F13 后门已封）
     B2 驳回必须带理由；重复审核 409；驳回后用户端状态为 rejected
  C. 支付宝渠道（第三方登录 + 实名核验）全链路
     C1 授权跳转 → 回调 → 票据换令牌 → 自动注册归入默认组 → 绑定行 + 主绑定快照
     C2 解绑最后一个登录方式被拒；恢复后解绑成功且快照清空
     C3 实名 authorize 拿 certify_id → 三方回调查结果 → 审核通过写入信任信号

关于支付宝：真实网关不可能在验收里连通，而这两条链路最需要被证明的恰恰是
「RSA2 签名与验签能不能互通」——只测配置错误分支等于没测。因此本脚本会编译
cmd/ucgatewaymock（一个只依赖标准库的支付宝网关模拟器）并挂进 backend_default
网络运行：它验平台的请求签名、回带签名的响应节点，让真实适配器代码完整跑通。
后端容器无法访问宿主机（INPUT 链 DROP），所以模拟器必须是与后端同网络的容器，
不能起在宿主机上。

前置：后端容器 backend-backend-1 与 postgres 已在跑（docker compose up -d）。
所有测试数据与渠道配置在脚本尾部还原（异常退出也走 finally）。
"""
import base64
import json
import os
import random
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

BASE = "http://127.0.0.1:8080/api/v1"
BACKEND_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__))) + "/backend"
PSQL = ["docker", "exec", "backend-postgres-1", "psql", "-U", "hostsent", "-d", "hostsent", "-tAc"]
FAILURES = []

MOCK_NAME = "uc-gw-mock"
MOCK_PORT = "18099"
MOCK_GATEWAY = f"http://{MOCK_NAME}:{MOCK_PORT}/gateway.do"
KEYS_DIR = "/tmp/ucmockkeys"
STATE_DIR = "/tmp/ucmockstate"
BIN_PATH = "/tmp/ucgatewaymock"

PWD = "Passw0rd!123"
suffix = str(random.randint(100000, 999999))

# 本脚本创建的用户 id，尾部统一清理。
created_users = []
# 渠道配置原值快照，尾部还原。
oauth_alipay_backup = None
realname_alipay_backup = None
provider_config_backup = None


# ---------------------------------------------------------------------------
# HTTP 工具
# ---------------------------------------------------------------------------

def req(method, path, token=None, body=None, timeout=60, redirect=True):
    """发一次请求，返回 (status, obj, headers)。obj 为解析后的 JSON 或原文。"""
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(BASE + path, data=data, method=method)
    r.add_header("Content-Type", "application/json")
    if token:
        r.add_header("Authorization", "Bearer " + token)
    if redirect:
        opener = urllib.request.build_opener()
    else:
        class NoRedirect(urllib.request.HTTPRedirectHandler):
            def redirect_request(self, *a, **k):
                return None

        opener = urllib.request.build_opener(NoRedirect)
    try:
        with opener.open(r, timeout=timeout) as resp:
            raw, status, headers = resp.read().decode(), resp.status, dict(resp.headers)
    except urllib.error.HTTPError as e:
        raw, status, headers = e.read().decode(), e.code, dict(e.headers)
    try:
        obj = json.loads(raw) if raw else None
    except json.JSONDecodeError:
        obj = raw
    return status, obj, headers


def dig(obj, key):
    """深度优先查找嵌套响应中的 key（只返回标量）。

    注意它**刻意跳过 dict/list 值**：这个函数是给「错误信封里的 code/message」
    这类形状不确定的场景用的。要取列表或对象请用 data()/path()，否则会静默拿到
    None —— 断言看起来「跑了」，其实什么都没校验。
    """
    if isinstance(obj, dict):
        if key in obj and not isinstance(obj[key], (dict, list)):
            return obj[key]
        for v in obj.values():
            found = dig(v, key)
            if found is not None:
                return found
    elif isinstance(obj, list):
        for v in obj:
            found = dig(v, key)
            if found is not None:
                return found
    return None


def data(obj):
    """取响应信封的 data（非 dict 时返回空 dict）。"""
    if isinstance(obj, dict) and isinstance(obj.get("data"), dict):
        return obj["data"]
    return {}


def data_list(obj):
    """取响应信封的 data 列表（非 list 时返回空列表）。"""
    if isinstance(obj, dict) and isinstance(obj.get("data"), list):
        return obj["data"]
    return []


def path(obj, *keys):
    """按固定路径取值（与 dig 不同，它不跳过 dict/list）。"""
    cur = obj
    for k in keys:
        if not isinstance(cur, dict):
            return None
        cur = cur.get(k)
    return cur


def query_param(url, name):
    q = urllib.parse.urlparse(url).query
    vals = urllib.parse.parse_qs(q).get(name)
    return vals[0] if vals else ""


def check(name, cond, detail=""):
    if not cond:
        FAILURES.append(name)
    print(f"  [{'PASS' if cond else 'FAIL'}] {name}{(' — ' + str(detail)) if detail else ''}")


def section(title):
    print(f"\n== {title} ==")


# ---------------------------------------------------------------------------
# 数据库 / 容器工具
# ---------------------------------------------------------------------------

def sql(query):
    """取单行单列（用于断言与快照）。"""
    out = subprocess.run(PSQL + [query], capture_output=True, text=True)
    return out.stdout.strip()


def sql_exec(query):
    return subprocess.run(PSQL + [query], capture_output=True, text=True)


def sh(args, **kw):
    return subprocess.run(args, capture_output=True, text=True, **kw)


def cleanup_user(uid):
    """删掉一个测试用户及其关联行。

    与 live 用例同款白名单：白名单看得见、出错易定位，也不会因将来新增一张表
    就误删真实业务数据。
    """
    if not uid:
        return
    app_ids = sql(f"SELECT COALESCE(string_agg(id::text, ','), '') FROM verification_applications WHERE user_id = {uid}")
    if app_ids:
        for table in ("verification_review_logs", "verification_documents", "verification_enterprises"):
            col = "application_id"
            sql_exec(f"DELETE FROM {table} WHERE {col} IN ({app_ids})")
    order_ids = sql(f"SELECT COALESCE(string_agg(id::text, ','), '') FROM orders WHERE user_id = {uid}")
    if order_ids:
        sql_exec(f"DELETE FROM order_items WHERE order_id IN ({order_ids})")
    pay_nos = sql(f"SELECT COALESCE(string_agg(payment_no, ','), '') FROM payment_orders WHERE user_id = {uid}")
    if pay_nos:
        quoted = ",".join("'" + p.strip() + "'" for p in pay_nos.split(",") if p.strip())
        sql_exec(f"DELETE FROM payment_callback_logs WHERE payment_no IN ({quoted})")
    for table in (
        "verification_applications", "user_oauth_bindings", "user_sessions", "login_logs",
        "user_roles", "sub_account_permissions", "wallet_accounts", "user_operation_logs",
        "staff_sales_relations", "orders", "payment_orders", "notifications",
        "sales_commission_transactions",
    ):
        col = "customer_user_id" if table == "sales_commission_transactions" else "user_id"
        sql_exec(f"DELETE FROM {table} WHERE {col} = {uid}")
    sql_exec(f"DELETE FROM users WHERE id = {uid}")


# ---------------------------------------------------------------------------
# 支付宝网关模拟器
# ---------------------------------------------------------------------------

def mock_keys():
    """生成两对 RSA 密钥：应用密钥（平台签请求）+ 模拟网关密钥（网关签响应）。"""
    os.makedirs(KEYS_DIR, exist_ok=True)
    os.makedirs(STATE_DIR, exist_ok=True)
    for f in os.listdir(KEYS_DIR):
        os.remove(os.path.join(KEYS_DIR, f))
    steps = [
        ["openssl", "genpkey", "-algorithm", "RSA", "-pkeyopt", "rsa_keygen_bits:2048",
         "-out", f"{KEYS_DIR}/app_private.pem"],
        ["openssl", "rsa", "-in", f"{KEYS_DIR}/app_private.pem", "-pubout",
         "-out", f"{KEYS_DIR}/app_public.pem"],
        ["openssl", "genpkey", "-algorithm", "RSA", "-pkeyopt", "rsa_keygen_bits:2048",
         "-out", f"{KEYS_DIR}/mock_private.pem"],
        ["openssl", "rsa", "-in", f"{KEYS_DIR}/mock_private.pem", "-pubout",
         "-out", f"{KEYS_DIR}/mock_public.pem"],
    ]
    for s in steps:
        out = sh(s)
        if out.returncode != 0:
            raise RuntimeError(f"生成密钥失败: {' '.join(s)} → {out.stderr}")
    return (open(f"{KEYS_DIR}/app_private.pem").read(),
            open(f"{KEYS_DIR}/app_public.pem").read(),
            open(f"{KEYS_DIR}/mock_public.pem").read())


def build_mock():
    """编译模拟器（静态链接，可直接挂进 alpine 容器运行）。"""
    env = dict(os.environ, CGO_ENABLED="0")
    out = sh(["go", "build", "-o", BIN_PATH, "./cmd/ucgatewaymock/"], cwd=BACKEND_DIR, env=env)
    if out.returncode != 0:
        raise RuntimeError(f"编译 ucgatewaymock 失败: {out.stderr}")


def start_mock():
    sh(["docker", "rm", "-f", MOCK_NAME])
    out = sh([
        "docker", "run", "-d", "--rm", "--name", MOCK_NAME, "--network", "backend_default",
        "-v", f"{BIN_PATH}:/app/ucgatewaymock:ro",
        "-v", f"{KEYS_DIR}:/keys:ro",
        "-v", f"{STATE_DIR}:/state",
        "alpine:3.20", "/app/ucgatewaymock",
        "-addr", ":" + MOCK_PORT,
        "-state", "/state/state.json",
        "-app-public-key", "/keys/app_public.pem",
        "-mock-private-key", "/keys/mock_private.pem",
    ])
    if out.returncode != 0:
        raise RuntimeError(f"启动模拟网关失败: {out.stderr}")
    # 后端容器通过 docker DNS 解析服务名，等它就绪（最多 5 秒）。
    for _ in range(10):
        time.sleep(0.5)
        probe = sh(["docker", "exec", "backend-backend-1", "sh", "-c",
                    f"wget -qO- --timeout=2 http://{MOCK_NAME}:{MOCK_PORT}/"])
        if probe.returncode == 0:
            return
    raise RuntimeError("模拟网关启动后不可达（检查 backend_default 网络与容器名）")


def stop_mock():
    sh(["docker", "rm", "-f", MOCK_NAME])


def mock_state():
    try:
        with open(f"{STATE_DIR}/state.json") as f:
            return json.load(f)
    except (OSError, json.JSONDecodeError):
        return {}


# ---------------------------------------------------------------------------
# 流程
# ---------------------------------------------------------------------------

def main():
    global oauth_alipay_backup, realname_alipay_backup, provider_config_backup

    section("0. 准备：管理员登录 + 快照渠道配置")
    st, obj, _ = req("POST", "/admin/auth/login", body={"username": "admin", "password": "123456"})
    admin = dig(obj, "token")
    check("管理员登录", st == 200 and bool(admin), f"HTTP {st} {obj if st != 200 else ''}")
    if not admin:
        return

    # 快照原值：脚本结束时按原样写回，避免「跑一次验收把渠道配置改了」。
    oauth_alipay_backup = sql(
        "SELECT COALESCE(json_agg(row_to_json(t))::text, '[]') FROM ("
        "SELECT provider, name, enabled, scopes, sort_order, remark, credentials "
        "FROM oauth_providers WHERE provider = 'alipay') t")
    realname_alipay_backup = sql(
        "SELECT COALESCE(json_agg(row_to_json(t))::text, '[]') FROM ("
        "SELECT id, provider_type, name, endpoint, priority, status, is_default, remark, credentials, descriptor "
        "FROM realname_providers WHERE provider_type = 'alipay') t")
    provider_config_backup = sql(
        "SELECT config_value FROM verification_configs WHERE config_key = 'verification.provider'")
    print(f"  已快照 oauth/realname 渠道配置与 verification.provider（原值 {provider_config_backup!r}）")

    # -----------------------------------------------------------------------
    section("A. 注销 / 回收站 / 恢复 / 批量")
    # -----------------------------------------------------------------------
    ua = f"zzclo{a_suffix()}"
    st, obj, _ = req("POST", "/uc/auth/register",
                     body={"username": ua, "password": PWD, "email": f"{ua}@example.com"})
    uid = dig(obj, "id")
    check("A0 注册测试用户", st == 200 and bool(uid), f"HTTP {st} {obj if st != 200 else ''}")
    created_users.append(uid)

    st, obj, _ = req("GET", f"/admin/users/{uid}/deletion-check", token=admin)
    check("A1 注销前置校验：新用户 can_delete=true",
          st == 200 and dig(obj, "can_delete") is True, f"HTTP {st} {obj if st != 200 else ''}")

    st, obj, _ = req("DELETE", f"/admin/users/{uid}", token=admin,
                     body={"reason": "e2e 用户体系验收", "force": True})
    check("A2 注销成功", st == 200 and dig(obj, "code") == 0, f"HTTP {st} {obj if st != 200 else ''}")
    row = sql(f"SELECT status || '|' || status_before_delete || '|' || (deleted_at IS NOT NULL)::text "
              f"FROM users WHERE id = {uid}")
    check("A2 注销后 status=cancelled、快照保留、deleted_at 非空", row == "cancelled|active|true", row)

    st, obj, _ = req("POST", "/uc/auth/login", body={"username": ua, "password": PWD})
    msg = str(dig(obj, "message") or "")
    check("A3 注销后登录被拒", st != 200 or dig(obj, "code") != 0, f"HTTP {st} {obj}")
    check("A3 提示为「用户名或密码错误」而非「账号已被禁用」（不泄露账号存在）",
          "禁用" not in msg and "错误" in msg, f"message={msg!r}")

    st, obj, _ = req("GET", f"/admin/users?filter=deleted&keyword={ua}&page=1&page_size=10", token=admin)
    items = path(obj, "data", "items") or []
    check("A4 回收站（filter=deleted）可见该用户", st == 200 and any(i.get("id") == uid for i in items),
          f"HTTP {st} 命中 {len(items)} 条，未见 id={uid}")

    # 默认列表必须把它排除掉：回收站是独立的视图，不是「列表多一列」。
    st, obj, _ = req("GET", f"/admin/users?keyword={ua}&page=1&page_size=10", token=admin)
    items = path(obj, "data", "items") or []
    check("A4 默认列表不含已注销用户", not any(i.get("id") == uid for i in items),
          f"默认列表命中 {len(items)} 条，含 id={uid}")

    ub = f"zzclo{a_suffix()}"
    st, obj, _ = req("POST", "/uc/auth/register",
                     body={"username": ub, "password": PWD, "email": f"{ub}@example.com"})
    reuse_id = dig(obj, "id")
    created_users.append(reuse_id)
    check("A5 注销后同 username 可重新注册（部分唯一索引只约束未注销行）",
          st == 200 and bool(reuse_id), f"HTTP {st} {obj if st != 200 else ''}")

    st, obj, _ = req("POST", f"/admin/users/{uid}/restore", token=admin)
    check("A6 账号名被占用时恢复明确返回 409 + 40904（不静默改名）",
          st == 409 and dig(obj, "code") == 40904, f"HTTP {st} {obj if st != 200 else ''}")

    # 清掉占用者后恢复应成功，且状态精确还原为注销前的 active。
    cleanup_user(reuse_id)
    created_users.remove(reuse_id)
    st, obj, _ = req("POST", f"/admin/users/{uid}/restore", token=admin)
    row = sql(f"SELECT status || '|' || (deleted_at IS NULL)::text FROM users WHERE id = {uid}")
    check("A7 清理占用后恢复成功且状态还原", st == 200 and row == "active|true", f"HTTP {st} row={row}")

    # 批量：造两个用户，批量注销 → 批量恢复。
    batch_ids = []
    for i in range(2):
        un = f"zzclo{suffix}b{i}"
        st, obj, _ = req("POST", "/uc/auth/register",
                         body={"username": un, "password": PWD, "email": f"{un}@example.com"})
        bid = dig(obj, "id")
        created_users.append(bid)
        batch_ids.append(bid)
    st, obj, _ = req("POST", "/admin/users/batch-delete", token=admin,
                     body={"ids": batch_ids, "reason": "e2e 批量注销", "force": True})
    affected = dig(obj, "affected")
    check("A8 批量注销 affected=2", st == 200 and affected == 2, f"HTTP {st} {obj if st != 200 else ''}")
    left = sql("SELECT count(*) FROM users WHERE id IN (%s) AND deleted_at IS NULL" % ",".join(map(str, batch_ids)))
    check("A8 批量注销后两行均已软删除", left == "0", f"未删除 {left} 行")

    st, obj, _ = req("POST", "/admin/users/batch-restore", token=admin, body={"ids": batch_ids})
    affected = dig(obj, "affected")
    left = sql("SELECT count(*) FROM users WHERE id IN (%s) AND deleted_at IS NULL" % ",".join(map(str, batch_ids)))
    check("A9 批量恢复 affected=2 且两行已还原",
          st == 200 and affected == 2 and left == "2", f"HTTP {st} affected={affected} 已还原 {left}")

    st, obj, _ = req("POST", "/admin/users/batch-delete", token=admin, body={"ids": [], "reason": "x"})
    check("A10 批量注销空 ids 被参数校验拒绝", st != 200, f"HTTP {st} {obj if st != 200 else ''}")

    # 留存期清理：dry_run 必须一个字节都不写。
    st, obj, _ = req("POST", "/admin/users/purge", token=admin, body={"dry_run": True, "limit": 5})
    check("A11 留存期清理 dry_run 回显 dry_run=true 且 retention_days 来自配置",
          st == 200 and dig(obj, "dry_run") is True and dig(obj, "retention_days") == 180,
          f"HTTP {st} {obj if st != 200 else ''}")
    alive = sql(f"SELECT (deleted_at IS NULL)::text FROM users WHERE id = {uid}")
    check("A11 dry_run 未改动任何用户行", alive == "true", f"deleted_at IS NULL = {alive}")

    # -----------------------------------------------------------------------
    section("B. 实名认证：提交 → 管理端审核 → 用户端状态")
    # -----------------------------------------------------------------------
    uv = f"zzclov{suffix}"
    st, obj, _ = req("POST", "/uc/auth/register",
                     body={"username": uv, "password": PWD, "email": f"{uv}@example.com"})
    vid = dig(obj, "id")
    created_users.append(vid)
    st, obj, _ = req("POST", "/uc/auth/login", body={"username": uv, "password": PWD})
    vtok = dig(obj, "token")
    check("B0 实名测试用户登录", st == 200 and bool(vtok), f"HTTP {st} {obj if st != 200 else ''}")

    st, obj, _ = req("GET", "/uc/verification", token=vtok)
    check("B1 初始实名状态 none", dig(obj, "status") == "none", f"HTTP {st} status={dig(obj, 'status')}")

    id_number = "110101199003071234"
    st, obj, _ = req("POST", "/uc/verification", token=vtok,
                     body={"verification_type": "personal", "real_name": "张三丰",
                           "id_number": id_number, "mobile": "13800138000"})
    app1 = dig(obj, "id")
    masked = dig(obj, "id_number_masked")
    check("B2 提交实名申请", st == 200 and bool(app1), f"HTTP {st} {obj if st != 200 else ''}")
    check("B2 证件号回显脱敏", masked and masked != id_number, f"masked={masked!r}")
    cipher = sql(f"SELECT id_number_encrypted FROM verification_applications WHERE id = {app1}")
    check("B2 证件号落密文（明文不入库）",
          bool(cipher) and id_number not in cipher, f"cipher={cipher[:24]!r}…")
    logs = sql(f"SELECT count(*) FROM verification_review_logs WHERE application_id = {app1} AND action = 'submit'")
    check("B2 提交写入一条 submit 审核轨迹", logs == "1", f"count={logs}")

    st, obj, _ = req("POST", "/uc/verification", token=vtok,
                     body={"verification_type": "personal", "real_name": "张三丰", "id_number": id_number})
    check("B3 重复提交（已有 pending）被拒", st != 200 or dig(obj, "code") != 0,
          f"HTTP {st} {obj if st != 200 else ''}")

    # 改展示名不得变成「已实名」——F13 后门的核心断言。
    st, obj, _ = req("PUT", "/uc/auth/profile", token=vtok, body={"name": "张三丰改名"})
    st2, obj2, _ = req("GET", "/uc/verification", token=vtok)
    signal = sql(f"SELECT (real_name_verified_at IS NOT NULL)::text FROM users WHERE id = {vid}")
    check("B4 改展示名不产生实名信任信号（real_name_verified_at 仍为空）",
          signal == "false" and dig(obj2, "status") == "pending",
          f"signal={signal} status={dig(obj2, 'status')}")

    st, obj, _ = req("POST", f"/admin/verifications/{app1}/approve", token=admin,
                     body={"note": "e2e 审核通过"})
    check("B5 管理端整单通过", st == 200 and dig(obj, "code") == 0, f"HTTP {st} {obj if st != 200 else ''}")
    sig = sql(f"SELECT COALESCE(real_name_verified_source, '') || '|' "
              f"|| (real_name_verified_at IS NOT NULL)::text FROM users WHERE id = {vid}")
    check("B5 通过后写入唯一信任信号（source=manual）", sig == "manual|true", sig)
    st, obj, _ = req("GET", "/uc/verification", token=vtok)
    check("B5 用户端状态变为 approved", dig(obj, "status") == "approved", f"status={dig(obj, 'status')}")

    st, obj, _ = req("POST", f"/admin/verifications/{app1}/approve", token=admin)
    check("B6 重复审核返回 409（整单状态迁移只允许 pending→approved）",
          st == 409, f"HTTP {st} {obj if st != 200 else ''}")

    # 驳回路径：另一个用户。
    ur = f"zzclor{suffix}"
    st, obj, _ = req("POST", "/uc/auth/register",
                     body={"username": ur, "password": PWD, "email": f"{ur}@example.com"})
    rid = dig(obj, "id")
    created_users.append(rid)
    st, obj, _ = req("POST", "/uc/auth/login", body={"username": ur, "password": PWD})
    rtok = dig(obj, "token")
    st, obj, _ = req("POST", "/uc/verification", token=rtok,
                     body={"verification_type": "personal", "real_name": "李四光",
                           "id_number": "11010119900307123X"})
    app2 = dig(obj, "id")
    st, obj, _ = req("POST", f"/admin/verifications/{app2}/reject", token=admin, body={})
    # 必须是 400 而不是 500：驳回理由为空是参数问题，落进 5xx 会让监控把它
    # 算成真实故障，前端也只能靠 message 文本猜「该不该让运营补理由」。
    check("B7 空理由的驳回被拒（400 参数错误，不是 500）",
          st == 400 and dig(obj, "code") == 20001, f"HTTP {st} {obj if st != 200 else ''}")
    st, obj, _ = req("POST", f"/admin/verifications/{app2}/reject", token=admin,
                     body={"reject_reason": "证件影像不清晰", "reject_reason_code": "blurred"})
    check("B8 带理由驳回成功", st == 200 and dig(obj, "code") == 0, f"HTTP {st} {obj if st != 200 else ''}")
    rsig = sql(f"SELECT (real_name_verified_at IS NOT NULL)::text FROM users WHERE id = {rid}")
    check("B8 被驳回用户不被标记为已实名", rsig == "false", f"signal={rsig}")
    st, obj, _ = req("GET", "/uc/verification", token=rtok)
    check("B8 用户端状态为 rejected 且带驳回理由",
          dig(obj, "status") == "rejected" and dig(obj, "reject_reason") == "证件影像不清晰",
          f"status={dig(obj, 'status')} reason={dig(obj, 'reject_reason')!r}")

    # -----------------------------------------------------------------------
    section("C. 支付宝渠道：第三方登录 + 实名核验（模拟网关）")
    # -----------------------------------------------------------------------
    app_priv, app_pub, mock_pub = mock_keys()
    build_mock()
    start_mock()
    print(f"  模拟网关已启动（{MOCK_GATEWAY}）")

    st, obj, _ = req("PUT", "/admin/oauth/providers/alipay", token=admin, body={
        "enabled": True, "scopes": "auth_user", "remark": "e2e 验收（模拟网关）",
        "credentials": {"app_id": "2021000000000000", "private_key": app_priv,
                        "alipay_public_key": mock_pub, "gateway_url": MOCK_GATEWAY,
                        "sign_type": "RSA2"},
    })
    check("C1 配置支付宝登录渠道（凭证字段级加密）",
          st == 200 and dig(obj, "enabled") is True, f"HTTP {st} {obj if st != 200 else ''}")
    shown = path(obj, "data", "credentials") or {}
    check("C1 凭证回显脱敏（不泄露私钥）",
          "enc:v1:" not in json.dumps(shown) and app_priv[:32] not in json.dumps(shown),
          f"credentials={shown}")

    st, obj, _ = req("POST", "/admin/oauth/providers/alipay/test", token=admin)
    ok = path(obj, "data", "ok")
    check("C1 连通性测试通过（平台请求签名被网关验签成功）",
          st == 200 and ok is True, f"HTTP {st} data={path(obj, 'data')}")

    # 渠道列表要能看出「刚测过且正常」：health_status 是运营判断渠道可用性的唯一入口。
    st, obj, _ = req("GET", "/admin/oauth/providers", token=admin)
    row = next((p for p in data_list(obj) if p.get("provider") == "alipay"), None)
    check("C1 连通性测试结果落库（health_status=healthy）",
          row is not None and row.get("health_status") == "healthy",
          f"health_status={row.get('health_status') if row else None}")

    st, obj, _ = req("GET", "/uc/oauth/providers")
    providers = [p.get("provider") for p in data_list(obj)]
    check("C2 用户端可见渠道列表含 alipay", "alipay" in providers, f"providers={providers}")
    leaked = json.dumps(obj)
    check("C2 公开渠道列表不含任何凭证字段",
          "app_id" not in leaked and "private_key" not in leaked and "enc:v1:" not in leaked)

    # —— 登录链路：授权 → 回调 → 票据换令牌 ——
    st, obj, _ = req("GET", "/uc/oauth/alipay/authorize")
    auth_url = dig(obj, "authorize_url") or ""
    state = query_param(auth_url, "state")
    check("C3 取到授权跳转地址且带签名 state", st == 200 and bool(state), f"HTTP {st} url={auth_url[:80]!r}")

    openid = f"e2eopenid{suffix}"
    st, obj, headers = req("GET",
                           f"/uc/oauth/alipay/callback?code={openid}&state={urllib.parse.quote(state)}",
                           redirect=False)
    location = headers.get("Location", "")
    check("C4 回调 302 回跳前端并带一次性票据",
          st == 302 and "ticket=" in location, f"HTTP {st} location={location[:80]!r}")
    check("C4 回调未返回 error 参数", "error=" not in location, location[:120])

    ticket = query_param(location, "ticket")
    st, obj, _ = req("POST", "/uc/oauth/exchange", body={"ticket": ticket})
    otok = dig(obj, "token")
    ouid = path(obj, "data", "user", "id")
    if ouid:
        created_users.append(ouid)
    check("C5 票据换正式令牌（自动注册）", st == 200 and bool(otok) and bool(ouid),
          f"HTTP {st} {obj if st != 200 else ''}")

    st, obj, _ = req("POST", "/uc/oauth/exchange", body={"ticket": ticket})
    check("C5 票据一次性（重复使用被拒）", st != 200, f"HTTP {st} {obj if st != 200 else ''}")

    if ouid:
        grp = sql(f"SELECT COALESCE(user_group_id::text, '') FROM users WHERE id = {ouid}")
        default_grp = sql("SELECT id::text FROM user_groups WHERE is_default = true AND status = 'active' LIMIT 1")
        check("C6 自动注册用户归入默认用户组", grp == default_grp and bool(grp), f"group={grp} 默认={default_grp}")
        binding = sql(f"SELECT count(*) FROM user_oauth_bindings WHERE user_id = {ouid} "
                      f"AND provider = 'alipay' AND status = 'active'")
        check("C6 写入绑定行", binding == "1", f"count={binding}")
        snap = sql(f"SELECT oauth_provider || '|' || oauth_openid FROM users WHERE id = {ouid}")
        check("C6 主绑定快照同步", snap.startswith("alipay|"), f"snapshot={snap}")
        sessions = sql(f"SELECT count(*) FROM user_sessions WHERE user_id = {ouid} AND status = 'active'")
        check("C6 第三方登录写入会话（F15：会话列表不再为空）", sessions != "0", f"count={sessions}")

    # —— 绑定 / 解绑守卫（用一个有密码的普通用户）——
    uo = f"zzcloo{suffix}"
    st, obj, _ = req("POST", "/uc/auth/register",
                     body={"username": uo, "password": PWD, "email": f"{uo}@example.com"})
    oid = dig(obj, "id")
    created_users.append(oid)
    st, obj, _ = req("POST", "/uc/auth/login", body={"username": uo, "password": PWD})
    otok2 = dig(obj, "token")

    st, obj, _ = req("GET", "/uc/oauth/alipay/bind-authorize", token=otok2)
    bind_state = query_param(dig(obj, "authorize_url") or "", "state")
    check("C7 已登录用户取到绑定授权地址", st == 200 and bool(bind_state), f"HTTP {st} {obj if st != 200 else ''}")
    bind_openid = f"e2ebind{suffix}"
    st, obj, headers = req("GET",
                           f"/uc/oauth/alipay/callback?code={bind_openid}&state={urllib.parse.quote(bind_state)}",
                           redirect=False)
    location = headers.get("Location", "")
    check("C7 绑定回调 302 且无 error", st == 302 and "error=" not in location, f"HTTP {st} {location[:100]!r}")
    bound = sql(f"SELECT count(*) FROM user_oauth_bindings WHERE user_id = {oid} AND provider = 'alipay'")
    check("C7 绑定行写入", bound == "1", f"count={bound}")

    st, obj, _ = req("GET", "/uc/oauth/bindings", token=otok2)
    mine = data_list(obj)
    entry = next((b for b in mine if b.get("provider") == "alipay"), None)
    check("C8 我的绑定列表可解绑预判 can_unbind=true（还有密码这一种方式）",
          entry is not None and entry.get("can_unbind") is True, f"bindings={mine}")

    sql_exec(f"UPDATE users SET password_hash = '', phone = '' WHERE id = {oid}")
    st, obj, _ = req("DELETE", "/uc/oauth/bindings/alipay", token=otok2)
    check("C9 解绑最后一个登录方式被拒（409）", st == 409, f"HTTP {st} {obj if st != 200 else ''}")
    still = sql(f"SELECT count(*) FROM user_oauth_bindings WHERE user_id = {oid} AND provider = 'alipay'")
    check("C9 被拒的解绑未删掉绑定行", still == "1", f"count={still}")

    sql_exec(f"UPDATE users SET password_hash = '$2a$10$e2eplaceholderhash' WHERE id = {oid}")
    st, obj, _ = req("DELETE", "/uc/oauth/bindings/alipay", token=otok2)
    snap = sql(f"SELECT COALESCE(oauth_provider, '') || COALESCE(oauth_openid, '') FROM users WHERE id = {oid}")
    check("C10 恢复登录方式后解绑成功且主绑定快照清空",
          st == 200 and snap == "", f"HTTP {st} snapshot={snap!r}")

    # —— 实名核验：跳转式认证 + 回调查结果 ——
    st, obj, _ = req("GET", "/admin/verifications/providers", token=admin)
    rn_list = data_list(obj)
    rn_row = next((p for p in rn_list if p.get("provider_type") == "alipay"), None)
    check("C11 已登记支付宝实名核验服务商（描述符模式 redirect）",
          rn_row is not None and rn_row.get("mode") == "redirect", f"providers={[p.get('provider_type') for p in rn_list]}")

    st, obj, _ = req("POST", "/admin/verifications/providers", token=admin, body={
        "id": rn_row["id"], "name": "支付宝实名认证", "status": 1, "is_default": True, "priority": 90,
        "credentials": {"app_id": "2021000000000000", "private_key": app_priv,
                        "alipay_public_key": mock_pub, "gateway_url": MOCK_GATEWAY,
                        "sign_type": "RSA2", "certify_mode": "FACE"},
    })
    check("C11 配置支付宝实名服务商并启用", st == 200 and dig(obj, "status") == 1,
          f"HTTP {st} {obj if st != 200 else ''}")

    st, obj, _ = req("POST", "/admin/verifications/configs", token=admin,
                     body={"config_key": "verification.provider", "config_value": "alipay"})
    check("C11 verification.provider 切到 alipay", st == 200, f"HTTP {st} {obj if st != 200 else ''}")

    ur2 = f"zzclorn{suffix}"
    st, obj, _ = req("POST", "/uc/auth/register",
                     body={"username": ur2, "password": PWD, "email": f"{ur2}@example.com"})
    rn_uid = dig(obj, "id")
    created_users.append(rn_uid)
    st, obj, _ = req("POST", "/uc/auth/login", body={"username": ur2, "password": PWD})
    rn_tok = dig(obj, "token")
    st, obj, _ = req("POST", "/uc/verification", token=rn_tok,
                     body={"verification_type": "personal", "real_name": "王五",
                           "id_number": "11010119900307123X", "mobile": "13800138000"})
    rn_app = dig(obj, "id")
    check("C12 提交实名申请（provider=alipay）", st == 200 and bool(rn_app),
          f"HTTP {st} {obj if st != 200 else ''}")

    st, obj, _ = req("POST", f"/uc/verification/applications/{rn_app}/authorize", token=rn_tok)
    auth_url = dig(obj, "auth_url") or ""
    txn_no = dig(obj, "txn_no") or ""
    check("C12 取到支付宝认证入口（跳转式，带 certify_id）",
          st == 200 and bool(txn_no) and MOCK_NAME in auth_url,
          f"HTTP {st} txn={txn_no!r} url={auth_url[:70]!r}")
    check("C12 certify_id 落库为流水号",
          sql(f"SELECT provider_txn_no FROM verification_applications WHERE id = {rn_app}") == txn_no,
          f"txn_no={txn_no!r}")

    st, obj, _ = req("GET",
                     f"/uc/verification/alipay/callback?application_id={rn_app}"
                     f"&certify_id={urllib.parse.quote(txn_no)}")
    check("C13 三方回调查询核验结果并落库", st == 200, f"HTTP {st} {obj if st != 200 else ''}")
    pres = sql(f"SELECT provider_result || '|' || COALESCE(provider_message, '') "
               f"FROM verification_applications WHERE id = {rn_app}")
    check("C13 provider_result=pass", pres.startswith("pass"), pres)
    logs = sql(f"SELECT count(*) FROM verification_review_logs WHERE application_id = {rn_app} "
               f"AND action = 'provider_pass'")
    check("C13 写入 provider_pass 审核轨迹", logs == "1", f"count={logs}")
    auto = sql(f"SELECT (real_name_verified_at IS NOT NULL)::text FROM users WHERE id = {rn_uid}")
    check("C13 自动放行默认关闭：核验通过不等于已实名", auto == "false", f"signal={auto}")

    st, obj, _ = req("GET", f"/uc/verification/applications/{rn_app}", token=rn_tok)
    check("C14 用户端可查本人申请详情", st == 200 and dig(obj, "id") == rn_app,
          f"HTTP {st} {obj if st != 200 else ''}")

    st, obj, _ = req("GET", f"/uc/verification/applications/{app1}", token=rn_tok)
    check("C14 越权查他人申请返回 404", st == 404, f"HTTP {st} {obj if st != 200 else ''}")

    st, obj, _ = req("POST", f"/admin/verifications/{rn_app}/approve", token=admin, body={"note": "e2e 核验后人工通过"})
    sig = sql(f"SELECT real_name_verified_source || '|' || (real_name_verified_at IS NOT NULL)::text "
              f"FROM users WHERE id = {rn_uid}")
    # source 记的是「这张单走的是哪家核验」而不是「谁点的通过」：申请单上
    # provider=alipay，因此来源为 alipay。人工审核的单子 provider 为空才回落 manual。
    check("C15 人工通过写入信任信号（来源=申请单上的 provider）",
          st == 200 and sig == "alipay|true", f"HTTP {st} {sig}")

    state = mock_state()
    calls = state.get("calls") or {}
    check("C16 平台请求签名被网关验签成功（RSA2 互通）",
          state.get("request_sign_ok") is True, f"sign_failures={state.get('sign_failures')}")
    check("C16 网关被调用的方法覆盖 oauth.token / oauth.info / certify.initialize / certify.query",
          all(calls.get(m, 0) >= 1 for m in (
              "alipay.system.oauth.token", "alipay.user.info.share",
              "alipay.user.certify.open.initialize", "alipay.user.certify.open.query")),
          f"calls={calls}")

    # 停掉网关后渠道应报「网关不可达」而不是「配置缺失」——错误分类不能混。
    stop_mock()
    st, obj, _ = req("POST", "/admin/oauth/providers/alipay/test", token=admin)
    msg = str(path(obj, "data", "message") or "")
    check("C17 网关不可达时连通性测试返回 ok=false 且原因为「不可达」",
          st == 200 and path(obj, "data", "ok") is False and "不可达" in msg,
          f"HTTP {st} data={path(obj, 'data')}")


def a_suffix():
    return f"{suffix}a"


# ---------------------------------------------------------------------------
# 收尾还原
# ---------------------------------------------------------------------------

def restore():
    stop_mock()
    if oauth_alipay_backup:
        try:
            rows = json.loads(oauth_alipay_backup)
        except json.JSONDecodeError:
            rows = []
        if rows:
            r = rows[0]
            run_sql("UPDATE oauth_providers SET name = %s, enabled = %s, scopes = %s, sort_order = %s, "
                    "remark = %s, credentials = %s::jsonb, updated_at = NOW() WHERE provider = 'alipay'"
                    % (quote(r["name"]), "true" if r["enabled"] else "false", quote(r["scopes"]),
                       int(r["sort_order"]), quote(r["remark"]), quote_json(r["credentials"])))
    if realname_alipay_backup:
        try:
            rows = json.loads(realname_alipay_backup)
        except json.JSONDecodeError:
            rows = []
        if rows:
            r = rows[0]
            run_sql("UPDATE realname_providers SET name = %s, endpoint = %s, priority = %s, status = %s, "
                    "is_default = %s, remark = %s, credentials = %s::jsonb, descriptor = %s::jsonb, "
                    "updated_at = NOW() WHERE id = %s"
                    % (quote(r["name"]), quote(r["endpoint"]), int(r["priority"]), int(r["status"]),
                       "true" if r["is_default"] else "false", quote(r["remark"]),
                       quote_json(r["credentials"]), quote_json(r["descriptor"] or {}), int(r["id"])))
    if provider_config_backup is not None:
        run_sql("UPDATE verification_configs SET config_value = %s WHERE config_key = 'verification.provider'"
                % quote(provider_config_backup))
    for uid in created_users:
        cleanup_user(uid)
    for path in (BIN_PATH,):
        if os.path.exists(path):
            os.remove(path)
    shutil.rmtree(STATE_DIR, ignore_errors=True)


def quote(v):
    """SQL 单引号字符串字面量（值来自库内快照或本脚本生成的密钥，无外部输入）。"""
    if v is None:
        return "NULL"
    return "'" + str(v).replace("'", "''") + "'"


def quote_json(v):
    """把 jsonb 列的快照值写回。

    row_to_json 已经把 jsonb 解成了 Python 对象（dict/list），直接 str() 会得到
    Python 的 repr —— 单引号、True/None 都不是合法 JSON，PG 会直接 22P02 报错，
    而还原语句又是「跑完就过」的收尾动作，不显式检查就会静默留下脏配置。
    """
    if v is None:
        return "'{}'"
    if isinstance(v, str):
        return quote(v)
    return quote(json.dumps(v, ensure_ascii=False))


def run_sql(query):
    """执行还原语句；失败必须可见（收尾静默失败等于没还原）。"""
    out = sql_exec(query)
    if out.returncode != 0:
        print(f"  !! 还原语句失败：{out.stderr.strip()[:200]}")
        FAILURES.append("收尾还原失败（见上方 stderr）")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:  # noqa: BLE001 — 脚本要把异常也计入失败并照常还原
        FAILURES.append(f"异常：{exc}")
        print(f"\n!! 执行中断：{exc}")
    finally:
        print("\n== 收尾：还原渠道配置 + 清理测试数据 ==")
        restore()
        left = sql("SELECT count(*) FROM users WHERE username LIKE 'zzclo%'")
        print(f"  残留 zzclo* 用户 {left} 行（期望 0）")

    print()
    if FAILURES:
        print(f"结果：{len(FAILURES)} 项失败 → {FAILURES}")
        sys.exit(1)
    print("结果：全部通过")
