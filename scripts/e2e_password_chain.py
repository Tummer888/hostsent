#!/usr/bin/env python3
"""密码留存链路端到端验收（对应迁移 100/101 与开通/查看密码改动）。

断言：
  1. 下单开通后，instances.credential_password 是 enc:v1: 密文（绝不是明文）；
  2. orders.credential_password 随单固化（重试复用同一密码的依据）；
  3. mock 上游实际收到的密码 == 「查看密码」接口返回的明文
     —— 这是本次修复的核心：此前密码只发上游不落库，查看永远取不到；
  4. 详情接口只报 has_password，绝不下发明文。

前置（脚本不负责创建）：
  - mock 上游容器 p5-mock 在 backend_default 网络、端口 9099，且已重启加载最新 mock.py
    （PUT /v1/clouds/{id}/password 与 /reinstall 用于验证重置/重装后的密码回写）；
  - 渠道 16（self，魔方云 mock）未被软删（deleted_at IS NULL）；
  - 商品 183「密码链路验收机」：self 链路、绑渠道 16，用于走带 rootpass 的开通表单。
"""
import json
import random
import subprocess
import sys
import time
import urllib.error
import urllib.request

BASE = "http://127.0.0.1:8080/api/v1"
PSQL = ["docker", "exec", "backend-postgres-1", "psql", "-U", "hostsent", "-d", "hostsent", "-tAc"]
MOCK = ["docker", "exec", "p5-mock", "cat", "/state/last_create.json"]

SELF_PRODUCT = 183  # 验收商品：self 链路，绑渠道 16 魔方云 mock（创建表单带 rootpass）
FAILURES = []


def req(method, path, token=None, body=None, timeout=60):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(BASE + path, data=data, method=method)
    r.add_header("Content-Type", "application/json")
    if token:
        r.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(r, timeout=timeout) as resp:
            raw, status = resp.read().decode(), resp.status
    except urllib.error.HTTPError as e:
        raw, status = e.read().decode(), e.code
    try:
        return status, (json.loads(raw) if raw else None)
    except json.JSONDecodeError:
        return status, raw


def dig(obj, key):
    """按 key 递归取值。credential 这类嵌套对象也返回（调用方自行判空）。"""
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


def sql(q):
    out = subprocess.run(PSQL + [q], capture_output=True, text=True)
    if out.returncode != 0:
        raise RuntimeError(out.stderr.strip())
    return out.stdout.strip()


def check(name, cond, detail=""):
    if not cond:
        FAILURES.append(name)
    print(f"  [{'PASS' if cond else 'FAIL'}] {name}{(' — ' + str(detail)) if detail else ''}")


def mock_form():
    out = subprocess.run(MOCK, capture_output=True, text=True)
    return json.loads(out.stdout or "{}").get("form", {})


suffix = str(random.randint(100000, 999999))
pwd = "Passw0rd!123"

st, obj = req("POST", "/admin/auth/login", body={"username": "admin", "password": "123456"})
admin = dig(obj, "token")
check("管理员登录", st == 200 and bool(admin), f"HTTP {st}")

u = f"pwd{suffix}"
req("POST", "/uc/auth/register", body={"username": u, "password": pwd, "email": f"{u}@example.com"})
st, obj = req("POST", "/uc/auth/login", body={"username": u, "password": pwd})
tok, uid = dig(obj, "token"), dig(obj, "id")
check("用户注册并登录", st == 200 and bool(tok), f"HTTP {st}")
st, obj = req("POST", "/admin/finance/transactions/adjust", token=admin, body={
    "user_id": uid, "type": "adjust", "direction": 1, "amount": 500,
    "biz_key": f"pwd-topup-{suffix}", "remark": "密码链路验收充值"})
check("充值", st == 200, f"HTTP {st} {obj}")

st, obj = req("POST", "/uc/orders", token=tok, body={"product_id": SELF_PRODUCT, "quantity": 1}, timeout=30)
order_id = dig(obj, "id")
check("下单成功", st == 200 and bool(order_id), f"HTTP {st} {obj}")

# 等开通完成
inst_id = None
deadline = time.time() + 120
while time.time() < deadline:
    row = sql(f"SELECT id,provider_instance_id,status FROM instances WHERE order_id={order_id} LIMIT 1")
    if row:
        parts = row.split("|")
        if parts[1]:
            inst_id = int(parts[0])
            break
    time.sleep(2)
check("实例已开通落库", inst_id is not None, f"order_id={order_id}")

if inst_id:
    # 1) 实例上的密码必须是密文
    stored = sql(f"SELECT credential_password FROM instances WHERE id={inst_id}")
    check("instances.credential_password 已留存", bool(stored), f"值={stored!r}")
    check("实例密码是 enc:v1: 密文（不落明文）", stored.startswith("enc:v1:"), f"值={stored!r}")

    # 2) 订单上的密码也已固化（重试复用同一密码的依据）
    order_pwd = sql(f"SELECT credential_password FROM orders WHERE id={order_id}")
    check("orders.credential_password 随单固化", order_pwd.startswith("enc:v1:"), f"值={order_pwd!r}")
    check("订单与实例留存的是同一份密文（不是各自新生成的）", order_pwd == stored,
          f"order={order_pwd[:24]}… inst={stored[:24]}…")

    # 3) 上游收到的密码 == 查看密码接口返回的明文（本次修复的核心断言）
    form = mock_form()
    sent = form.get("rootpass", "")
    check("mock 上游收到了密码（rootpass）", bool(sent), f"rootpass={sent!r}")

    st, obj = req("POST", f"/admin/instances/{inst_id}/credential", token=admin)
    revealed = dig(obj, "password")
    check("查看密码接口返回成功", st == 200 and bool(revealed), f"HTTP {st} {obj}")
    check("查看密码 == 上游实际收到的密码", revealed == sent,
          f"reveal={revealed!r} upstream={sent!r}")

    # 4) 详情接口绝不下发明文
    st, obj = req("GET", f"/admin/instances/{inst_id}", token=admin)
    cred = dig(obj, "credential") or {}
    body = json.dumps(obj, ensure_ascii=False)
    check("详情报 has_password=true", cred.get("has_password") is True, f"credential={cred}")
    check("详情响应里不含明文密码", sent not in body if sent else True, "OK")

    # 5) 审计流水留痕
    ops = sql(f"SELECT count(*) FROM instance_operations WHERE instance_id={inst_id} AND action='reveal_credential'")
    check("查看密码写入审计流水", int(ops or 0) >= 1, f"流水条数={ops}")

print()
if FAILURES:
    print(f"FAILED: {len(FAILURES)} 项 -> {FAILURES}")
    sys.exit(1)
print("ALL PASS")
