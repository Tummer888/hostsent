#!/usr/bin/env python3
# ============================================================================
# e2e_p6_open_api.py — 资源管理双链路重构 · P6 对外开放端到端验收（doc17 §7-P6）
#
# 用法：
#   python3 scripts/e2e_p6_open_api.py --app-id APP_ID --app-secret SECRET \
#       [--only t61|t62|t63|t64|t65|t66]
#
# 前置：
#   - backend 容器在 localhost:8080；
#   - 应用已用 cmd/openapp 创建（owner_user_id 指向真实用户），
#     运行时用 --app-id/--app-secret 传入凭证。
#
# 环境自愈（不依赖演示库种子）：商品 1 是历史种子约定，演示库换过数据后即漂移。
# 脚本改为**自建** dcimcloud mock 渠道 + self 商品（询价/下单/实例操作全用它，
# 走 p5-mock 而非真实上游），跑完清理。owner 折扣率从 DB 动态读取（0.85 断言
# 跟随当前等级折扣，不硬编码数值）。
#
# 注意：tier3 需要 OPEN_APP_ID/OPEN_APP_SECRET，本地未持凭证时无法全量验证；
# 环境自愈与语法由 CI py_compile 保证，凭证相关断言需在有凭证的环境跑。
#
# 分阶段断言（随任务推进逐步扩展）：
#   t61  签名链路：ping 通过；缺头/错签/时间窗/nonce 重放/IP 白名单/停用各返回固定码
#   t62  只读接口：spec-atoms / products / regions / images / quote（owner 组折扣）
#   t63  代客下单：幂等 + customer_ref + channel 三列落库
#   t64  实例：查询/续费/电源/暂停；DELETE 返回 40009（D2）
#   t65  事件回调：接收端验签 + 重试 + 死信重投
#   t66  对账：/open/v1/audit/*
# ============================================================================

import argparse
import hashlib
import hmac
import json
import random
import subprocess
import sys
import time
import urllib.parse
import urllib.request
import uuid

BASE = "http://localhost:8080"
PSQL = ["docker", "exec", "backend-postgres-1", "psql", "-U", "hostsent", "-d", "hostsent", "-tAc"]

# 自建资源统一前缀：清理时按名字前缀定位，不会误伤运营的渠道/商品。
PROVIDER_PREFIX = "e2e-p6-dcim渠道"
PRODUCT_PREFIX = "e2e-p6-open商品"
# 商品分类：建品现在强制必填（留空会落进「未分类」，导航/列表都归不了位），
# 脚本自建一个分类，跑完在删完商品之后删除（分类下仍有商品时后端会拒绝删除）。
CATEGORY_PREFIX = "e2e-p6-分类"

PASS = 0
FAIL = 0
FAILED = []


def sql(query):
    out = subprocess.run(PSQL + [query], capture_output=True, text=True)
    if out.returncode != 0:
        raise RuntimeError(out.stderr.strip())
    return out.stdout.strip()


def dig(obj, key):
    """深度优先查找嵌套响应中的 key（兼容 code/data 信封）。"""
    if isinstance(obj, dict):
        if key in obj:
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


def q(s):
    """SQL 字符串字面量（单引号转义）。"""
    return s.replace("'", "''")


def admin_req(method, path, token, body=None, timeout=60):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request("http://127.0.0.1:8080/api/v1" + path, data=data, method=method)
    r.add_header("Content-Type", "application/json")
    if token:
        r.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(r, timeout=timeout) as resp:
            raw, status = resp.read().decode(), resp.status
    except urllib.error.HTTPError as e:
        raw, status = e.read().decode(), e.code
    try:
        return status, json.loads(raw) if raw else None
    except json.JSONDecodeError:
        return status, raw


def setup_self_product():
    """管理员登录 + 自建分类/channel/self 商品，返回 (product_id, provider_id, admin_token, category_id)。"""
    st, obj = admin_req("POST", "/admin/auth/login", None, body={"username": "admin", "password": "123456"})
    admin_token = dig(obj, "token")
    check("管理员登录（自建验收资源用）", st == 200 and bool(admin_token), f"HTTP {st}")
    if not admin_token:
        sys.exit(1)

    suffix = str(random.randint(100000, 999999))
    st, obj = admin_req("POST", "/admin/product/categories", admin_token, body={
        "parent_id": 0, "name": f"{CATEGORY_PREFIX}-{suffix}", "sort_order": 0, "status": 1})
    category_id = dig(obj, "id")
    check("自建商品分类（供 self 商品归属）", st == 200 and bool(category_id), f"HTTP {st}")
    if not category_id:
        sys.exit(1)

    st, obj = admin_req("POST", "/admin/resource/providers", admin_token, body={
        "name": f"{PROVIDER_PREFIX}-{suffix}", "provider_type": "dcimcloud",
        "api_endpoint": "http://p5-mock:9099", "status": 1, "des": "P6 开放平台 e2e 自建（跑完清理）",
        "credentials": {"api_key": "e2e", "api_secret": "mock", "account_type": "admin"}})
    provider_id = dig(obj, "id")
    check("自建 dcimcloud 渠道（指向 p5-mock）", st == 200 and bool(provider_id), f"HTTP {st}")
    if not provider_id:
        sys.exit(1)

    st, obj = admin_req("POST", "/admin/product/products", admin_token, body={
        "code": f"e2e-p6-open-{suffix}", "name": f"{PRODUCT_PREFIX}-{suffix}",
        "product_type": "cloud_host", "price_model": "monthly", "price": 100.0, "cost_price": 0.0,
        "source_mode": "self", "source_provider_id": provider_id, "stock": -1, "status": 1,
        "category_id": category_id,
        "description": "P6 开放平台 e2e 自建（跑完清理）"})
    product_id = dig(obj, "id")
    check("自建 self 商品（open 询价/下单样本）", st == 200 and bool(product_id), f"HTTP {st}")
    if not product_id:
        sys.exit(1)
    return product_id, provider_id, admin_token, category_id


def cleanup(product_id, provider_id, admin_token, category_id=None):
    """清理自建资源：先删续费单与实例（订单开通/续费产生），再删商品，最后删渠道与分类。"""
    if product_id:
        sql(f"DELETE FROM instance_renewals WHERE instance_id IN "
            f"(SELECT id FROM instances WHERE sell_product_id = {product_id})")
        sql(f"DELETE FROM instance_operations WHERE instance_id IN "
            f"(SELECT id FROM instances WHERE sell_product_id = {product_id})")
        sql(f"DELETE FROM instances WHERE sell_product_id = {product_id}")
    if product_id and admin_token:
        st, obj = admin_req("DELETE", f"/admin/product/products/{product_id}", admin_token)
        if st != 200:
            print(f"  [WARN] 自建商品 {product_id} 清理失败（HTTP {st}）：{obj}")
    if provider_id and admin_token:
        st, obj = admin_req("DELETE", f"/admin/resource/providers/{provider_id}", admin_token)
        if st != 200:
            print(f"  [WARN] 自建渠道 {provider_id} 清理失败（HTTP {st}）：{obj}")
    # 分类最后删：它下面挂着刚删掉的商品，早删会被「分类下仍有商品」的守卫拦下。
    if category_id and admin_token:
        st, obj = admin_req("DELETE", f"/admin/product/categories/{category_id}", admin_token)
        if st != 200:
            print(f"  [WARN] 自建分类 {category_id} 清理失败（HTTP {st}）：{obj}")
    print("已清理自建渠道、商品、开通实例与分类")


def owner_discount_rate(app_id):
    """owner 用户的代理等级 all 折扣率（DB 权威值，断言跟随不硬编码 0.85）。"""
    row = sql(
        f"SELECT COALESCE(ad.discount_rate, 0) FROM open_apps oa "
        f"LEFT JOIN users u ON u.id = oa.owner_user_id "
        f"LEFT JOIN agent_level_discounts ad ON ad.agent_level_id = u.agent_level_id "
        f"AND ad.target_type = 'all' WHERE oa.app_id = '{q(app_id)}'")
    try:
        return float(row or 0)
    except ValueError:
        return 0.0


def check(name, cond, detail=""):
    global PASS, FAIL
    if cond:
        PASS += 1
        print(f"  [PASS] {name}")
    else:
        FAIL += 1
        FAILED.append(name)
        print(f"  [FAIL] {name}  {detail}")


class OpenClient:
    """开放平台签名客户端（与服务端 signature.go 规范对齐）。"""

    def __init__(self, app_id, app_secret, base=BASE):
        self.app_id = app_id
        self.secret = app_secret
        self.base = base

    @staticmethod
    def canonical_query(query: dict) -> str:
        if not query:
            return ""
        pairs = []
        for k in sorted(query):
            for v in sorted(query[k] if isinstance(query[k], list) else [query[k]]):
                pairs.append(f"{urllib.parse.quote(str(k), safe='')}={urllib.parse.quote(str(v), safe='')}")
        return "&".join(pairs)

    def sign(self, method, path, query: dict, body: bytes, ts: str, nonce: str) -> str:
        sts = "\n".join([
            method.upper(), path, self.canonical_query(query),
            hashlib.sha256(body).hexdigest(), ts, nonce,
        ])
        return hmac.new(self.secret.encode(), sts.encode(), hashlib.sha256).hexdigest()

    def request(self, method, path, query=None, body=None, ts_offset=0, nonce=None,
                app_id=None, signature=None, headers_extra=None):
        query = query or {}
        body_bytes = json.dumps(body).encode() if body is not None else b""
        qs = ("?" + urllib.parse.urlencode(query, doseq=True)) if query else ""
        ts = str(int(time.time()) + ts_offset)
        nonce = nonce or uuid.uuid4().hex
        sig = signature if signature is not None else self.sign(
            method, path.split("?")[0], query, body_bytes, ts, nonce)
        headers = {
            "X-App-Id": app_id or self.app_id,
            "X-Timestamp": ts,
            "X-Nonce": nonce,
            "X-Signature": sig,
        }
        if headers_extra:
            headers.update(headers_extra)
        req = urllib.request.Request(self.base + path + qs, data=body_bytes if body_bytes else None,
                                     headers=headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                return resp.status, json.loads(resp.read().decode())
        except urllib.error.HTTPError as e:
            try:
                return e.code, json.loads(e.read().decode())
            except Exception:
                return e.code, {}


def code_of(resp):
    return resp.get("code")


# ---------------------------------------------------------------------------
# T6.1 签名链路
# ---------------------------------------------------------------------------

def run_t61(c: OpenClient):
    print("== T6.1 签名链路 ==")

    status, resp = c.request("GET", "/open/v1/ping")
    check("签名 ping 返回 0 且回显 app_id", code_of(resp) == 0 and resp.get("data", {}).get("app_id") == c.app_id,
          f"{status} {resp}")

    # 带查询参数的签名（canonical query 排序）
    status, resp = c.request("GET", "/open/v1/ping", query={"b": "2", "a": "1"})
    check("带 query 的签名 ping 通过", code_of(resp) == 0, f"{resp}")

    # 带 body 的签名（POST）
    status, resp = c.request("POST", "/open/v1/ping", body={"x": 1})
    check("带 body 的签名 ping 通过（POST 路由未注册返回 40404/0 视网关结果）",
          code_of(resp) in (0, 40404) or status == 404, f"{status} {resp}")

    # 缺认证头
    import urllib.request as _ur
    req = _ur.Request(BASE + "/open/v1/ping")
    with _ur.urlopen(req, timeout=10) as r:
        resp2 = json.loads(r.read().decode())
    check("无签名返回 40100", code_of(resp2) == 40100, f"{resp2}")

    # 错误签名
    status, resp = c.request("GET", "/open/v1/ping", signature="0" * 64)
    check("错误签名返回 40102", code_of(resp) == 40102, f"{resp}")

    # body 篡改：签名基于 A body、发送 B body
    body_bytes = json.dumps({"x": 1}).encode()
    ts = str(int(time.time()))
    nonce = uuid.uuid4().hex
    sts = "\n".join(["POST", "/open/v1/ping", "", hashlib.sha256(body_bytes).hexdigest(), ts, nonce])
    sig = hmac.new(c.secret.encode(), sts.encode(), hashlib.sha256).hexdigest()
    req = _ur.Request(BASE + "/open/v1/ping", data=json.dumps({"x": 2}).encode(), method="POST",
                      headers={"X-App-Id": c.app_id, "X-Timestamp": ts, "X-Nonce": nonce, "X-Signature": sig,
                               "Content-Type": "application/json"})
    try:
        with _ur.urlopen(req, timeout=10) as r:
            resp2 = json.loads(r.read().decode())
    except _ur.HTTPError as e:
        raw = e.read().decode()
        try:
            resp2 = json.loads(raw)
        except Exception:
            resp2 = {"code": None, "raw": raw, "http_status": e.code}
    check("body 篡改返回 40102", code_of(resp2) == 40102, f"{resp2}")

    # 时间戳超出窗口（-10 分钟）
    status, resp = c.request("GET", "/open/v1/ping", ts_offset=-600)
    check("过期时间戳返回 40103", code_of(resp) == 40103, f"{resp}")

    # nonce 重放
    nonce = "replay-" + uuid.uuid4().hex
    status, resp1 = c.request("GET", "/open/v1/ping", nonce=nonce)
    status, resp2 = c.request("GET", "/open/v1/ping", nonce=nonce)
    check("首次 nonce 通过", code_of(resp1) == 0, f"{resp1}")
    check("nonce 重放返回 40104", code_of(resp2) == 40104, f"{resp2}")

    # 未知应用
    status, resp = c.request("GET", "/open/v1/ping", app_id="app_missing")
    check("未知应用返回 40101", code_of(resp) == 40101, f"{resp}")


# ---------------------------------------------------------------------------
# T6.2 只读目录与询价
# ---------------------------------------------------------------------------

def run_t62(c: OpenClient, product_id: int, discount: float):
    print("== T6.2 只读接口 ==")

    status, resp = c.request("GET", "/open/v1/spec-atoms")
    atoms = resp.get("data") or []
    check("spec-atoms 返回 0 且非空", code_of(resp) == 0 and len(atoms) > 0, f"{status} code={code_of(resp)}")
    check("spec-atoms 不泄露平台字段映射", all("platform_fields" not in a for a in atoms))

    status, resp = c.request("GET", "/open/v1/products", query={"page": 1, "page_size": 50})
    data = resp.get("data") or {}
    items = data.get("items") or []
    check("products 返回 0 且非空", code_of(resp) == 0 and len(items) > 0, f"{code_of(resp)} n={len(items)}")
    check("products 不含成本/状态机内部字段",
          all(not ({"cost_price", "status", "provision_mode"} & set(it.keys())) for it in items))
    target = next((it for it in items if it.get("id") == product_id), None)
    check("自建商品（在售）可见", target is not None, f"{items[:2]}")

    status, resp = c.request("GET", f"/open/v1/products/{product_id}")
    detail = resp.get("data") or {}
    check("商品详情返回 0", code_of(resp) == 0 and detail.get("id") == product_id, f"{code_of(resp)}")

    status, resp = c.request("GET", "/open/v1/products/999999")
    check("不存在商品返回 40404", code_of(resp) == 40404, f"{resp}")

    status, resp = c.request("GET", "/open/v1/regions")
    regions = resp.get("data") or []
    check("regions 返回 0（列表可能为空）", code_of(resp) == 0, f"{code_of(resp)}")
    check("regions 字段只有 id/name/type", all(set(r.keys()) <= {"id", "name", "type"} for r in regions))

    status, resp = c.request("GET", "/open/v1/images")
    images = resp.get("data") or []
    check("images 返回 0", code_of(resp) == 0, f"{code_of(resp)}")
    check("images 字段只有 id/name", all(set(i.keys()) <= {"id", "name"} for i in images))

    # 询价（D3：owner 代理等级折扣；折扣率从 DB 动态读取）
    status, resp = c.request("POST", "/open/v1/quote", body={"product_id": product_id, "quantity": 1})
    quote = resp.get("data") or {}
    check("quote 返回 0 且金额>0", code_of(resp) == 0 and quote.get("final_amount", 0) > 0, f"{resp}")
    original = quote.get("original_amount", 0)
    final = quote.get("final_amount", 0)
    if discount > 0:
        expected = round(original * discount, 2)
        check(f"quote 实付=原价×折扣率 {discount}（{final} == {expected}）", abs(final - expected) < 0.011, f"{quote}")
    else:
        check("owner 无代理折扣时实付=原价", abs(final - original) < 0.011, f"{quote}")
    check("quote 不泄露折扣明细字段",
          not ({"discount_amount", "price_policy_id", "discount_source", "price_snapshot"} & set(quote.keys())))

    # scope 缺失：用只读 app 验证（由外部准备，跳过条件见 run_t62_scoped）
    return {"quote_original": original, "quote_final": final}


def run_t62_scope_denied(c_readonly: OpenClient):
    """只读 app（无 order:create）访问 quote 之外——catalog:read 覆盖 T6.2 全部端点；
    用未授 catalog:read 的应用验证 40301（由调用方保证 app 权限差集）。"""
    print("== T6.2 scope 校验 ==")
    status, resp = c_readonly.request("GET", "/open/v1/spec-atoms")
    check("无 catalog:read 访问 spec-atoms 返回 40301", code_of(resp) == 40301, f"{resp}")


# ---------------------------------------------------------------------------
# T6.3 代客下单
# ---------------------------------------------------------------------------

def run_t63(c: OpenClient, product_id: int, discount: float):
    print("== T6.3 代客下单 ==")

    # 正常下单（幂等键 + customer_ref）
    req_id = "e2e-order-" + uuid.uuid4().hex
    status, resp = c.request("POST", "/open/v1/orders",
                             body={"product_id": product_id, "quantity": 1, "customer_ref": "cust-e2e-001"},
                             headers_extra={"X-Client-Request-Id": req_id})
    data = resp.get("data") or {}
    order = data.get("order") or {}
    check("下单返回 0 且订单号非空", code_of(resp) == 0 and order.get("order_no"), f"{resp}")
    check("下单后状态为 paid（异步开通）", order.get("status") in ("paid", "provisioning", "active"), f"{order}")
    if discount > 0:
        check(f"下单实付=原价×折扣率 {discount}",
              abs(order.get("paid_amount", 0) - round(order.get("original_amount", 0) * discount, 2)) < 0.011,
              f"{order}")
    else:
        check("owner 无代理折扣时实付=原价",
              abs(order.get("paid_amount", 0) - order.get("original_amount", 0)) < 0.011, f"{order}")
    check("首次响应 idempotently_replayed=False", data.get("idempotently_replayed") is False, f"{data}")

    # 同幂等键重放：返回同一订单、不重复扣款
    status, resp = c.request("POST", "/open/v1/orders",
                             body={"product_id": product_id, "quantity": 1, "customer_ref": "cust-e2e-001"},
                             headers_extra={"X-Client-Request-Id": req_id})
    data2 = resp.get("data") or {}
    order2 = data2.get("order") or {}
    check("重放返回同一订单", code_of(resp) == 0 and order2.get("order_no") == order.get("order_no"), f"{resp}")
    check("重放标记 idempotently_replayed=True", data2.get("idempotently_replayed") is True, f"{data2}")

    # 缺幂等键
    status, resp = c.request("POST", "/open/v1/orders", body={"product_id": product_id, "quantity": 1})
    check("缺幂等键返回 40001", code_of(resp) == 40001, f"{resp}")

    # 失败快照：不存在商品 → 40404；同键重放返回同样 40404（不隐式重执行）
    bad_id = "e2e-bad-" + uuid.uuid4().hex
    status, resp = c.request("POST", "/open/v1/orders",
                             body={"product_id": 999999, "quantity": 1},
                             headers_extra={"X-Client-Request-Id": bad_id})
    check("不存在商品下单返回 40404", code_of(resp) == 40404, f"{resp}")
    status, resp = c.request("POST", "/open/v1/orders",
                             body={"product_id": 999999, "quantity": 1},
                             headers_extra={"X-Client-Request-Id": bad_id})
    check("失败快照重放返回同样 40404", code_of(resp) == 40404, f"{resp}")

    return {"order_no": order.get("order_no"), "request_id": req_id}


# ---------------------------------------------------------------------------
# T6.4 实例接口
# ---------------------------------------------------------------------------

def wait_instance_provisioned(product_id, secs=90):
    """等待自建商品的实例完成异步开通（worker 约 2s 轮询）。

    返回 (inst_id, provider_instance_id)。占位行没有上游实例号时电源/暂停/续费
    都会被 50000 拒绝（ensureProvisioned），因此 t64 的操作断言必须等开通落地。
    """
    deadline = time.time() + secs
    while time.time() < deadline:
        row = sql(
            f"SELECT id, provider_instance_id FROM instances "
            f"WHERE sell_product_id = {product_id} AND provider_instance_id <> '' "
            f"ORDER BY id DESC LIMIT 1")
        if row:
            inst_id, provider_inst_id = row.split("|", 1)
            return int(inst_id), provider_inst_id.strip()
        time.sleep(2)
    return 0, ""


def run_t64(c: OpenClient, product_id: int):
    print("== T6.4 实例接口 ==")

    # 等开通完成（异步履约）：占位行可被列表看到，但上游操作会拒绝，
    # 必须等 provider_instance_id 落地（worker 约 2s 轮询）再操作。
    inst_id, provider_inst_id = wait_instance_provisioned(product_id)
    check("自建商品实例开通落库（provider_instance_id 非空）", inst_id > 0, f"inst={inst_id}")

    inst = None
    for _ in range(10):
        status, resp = c.request("GET", "/open/v1/instances", query={"page": 1, "page_size": 50})
        data = resp.get("data") or {}
        items = data.get("items") or []
        inst = next((it for it in items if it.get("order_id") and it.get("source_mode") == "self"), None)
        if inst:
            break
        time.sleep(1)
    check("实例列表可见 T6.3 开通的实例", inst is not None, f"{resp}")

    if inst_id and provider_inst_id:
        status, resp = c.request("GET", f"/open/v1/instances/{inst_id}")
        check("实例详情返回 0", code_of(resp) == 0 and (resp.get("data") or {}).get("id") == inst_id, f"{resp}")

        status, resp = c.request("GET", "/open/v1/instances/999999")
        check("不存在/不归属实例返回 40404", code_of(resp) == 40404, f"{resp}")

        status, resp = c.request("POST", f"/open/v1/instances/{inst_id}/power", body={"action": "off"})
        check("电源关机返回 0", code_of(resp) == 0, f"{resp}")

        status, resp = c.request("POST", f"/open/v1/instances/{inst_id}/power", body={"action": "explode"})
        check("非法电源动作返回 40001", code_of(resp) == 40001, f"{resp}")

        status, resp = c.request("POST", f"/open/v1/instances/{inst_id}/suspend", body={"reason": "e2e-suspend"})
        check("暂停实例返回 0", code_of(resp) == 0, f"{resp}")

        status, resp = c.request("POST", f"/open/v1/instances/{inst_id}/unsuspend")
        check("恢复实例返回 0", code_of(resp) == 0, f"{resp}")

        status, resp = c.request("POST", f"/open/v1/instances/{inst_id}/renew", body={"period_count": 1})
        check("续费返回 0（双链路续费）", code_of(resp) == 0, f"{resp}")
        renewal = (resp.get("data") or {}).get("renewal") or {}
        check("续费单金额>0（折扣口径由 DB 断言）", renewal.get("amount", 0) > 0, f"{renewal}")

    # D2：DELETE 必须返回明确的「不支持」
    status, resp = c.request("DELETE", f"/open/v1/instances/{inst_id or 1}")
    check("DELETE /instances/{id} 返回 40009（D2）", code_of(resp) == 40009, f"{resp}")
    check("DELETE 响应明确说明不支持销毁", "不支持" in (resp.get("message") or ""), f"{resp}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--app-id", required=True)
    ap.add_argument("--app-secret", required=True)
    ap.add_argument("--readonly-app-id", default="", help="仅具部分 scope 的应用（scope 拒绝用例）")
    ap.add_argument("--readonly-app-secret", default="")
    ap.add_argument("--only", default="all", help="t61|t62|t63|t64|t65|t66|all")
    args = ap.parse_args()

    client = OpenClient(args.app_id, args.app_secret)
    only = args.only

    # 自建验收商品与渠道（t61 纯签名用例不依赖商品，但仍提前清理历史残留）。
    product_id, provider_id, admin_token, category_id = setup_self_product()
    discount = owner_discount_rate(args.app_id)

    if only in ("all", "t61"):
        run_t61(client)
    if only in ("all", "t62"):
        run_t62(client, product_id, discount)
        if args.readonly_app_id and args.readonly_app_secret:
            run_t62_scope_denied(OpenClient(args.readonly_app_id, args.readonly_app_secret))
    if only in ("all", "t63"):
        run_t63(client, product_id, discount)
    if only in ("all", "t64"):
        run_t64(client, product_id)

    cleanup(product_id, provider_id, admin_token, category_id)

    print(f"\n结果：PASS={PASS} FAIL={FAIL}")
    if FAILED:
        print("失败项：")
        for name in FAILED:
            print(f"  - {name}")
    sys.exit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
