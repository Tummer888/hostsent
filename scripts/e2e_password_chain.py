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
    （PUT /v1/clouds/{id}/password 与 /reinstall 用于验证重置/重装后的密码回写）。

自愈（不依赖演示库种子）：渠道 16/商品 183 是历史种子约定，演示库换过数据后
即漂移。脚本改为**自建**同名资源并跑完清理：
  - 自建 dcimcloud 渠道（指向 p5-mock，账号密码走动态凭证）；
  - 自建 self 商品（config_options 带 rootpass 走开通表单，未挂规格）；
  - 跑完删除自建商品与渠道（渠道删除前需无关联商品，见清理顺序）。
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

# 自建资源统一前缀：清理时按名字前缀定位，不会误伤运营的渠道/商品。
PROVIDER_PREFIX = "e2e-pwd-mock渠道"
PRODUCT_PREFIX = "e2e-pwd-验收机"
# 商品分类：建品现在强制必填（留空会让商品落进「未分类」，导航与列表都归不了位），
# 脚本自建一个分类，跑完在删完商品之后删除（分类下还有商品时后端会拒绝删除）。
CATEGORY_PREFIX = "e2e-pwd-分类"
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

def cleanup_category(cid):
    """删除自建分类：必须在删商品之后调用（分类下仍有商品时后端拒绝删除）。"""
    if not cid:
        return
    st, obj = req("DELETE", f"/admin/product/categories/{cid}", token=admin)
    if st != 200:
        print(f"  [WARN] 自建分类 {cid} 清理失败（HTTP {st}）：{obj}")


print("== 0. 自建分类 + dcimcloud 渠道 + self 商品 ==")
st, obj = req("POST", "/admin/product/categories", token=admin, body={
    "parent_id": 0, "name": f"{CATEGORY_PREFIX}-{suffix}", "sort_order": 0, "status": 1})
category_id = dig(obj, "id")
check("自建商品分类（供 self 商品归属）", st == 200 and bool(category_id), f"HTTP {st} {obj if st != 200 else ''}")
if not category_id:
    sys.exit(1)

st, obj = req("POST", "/admin/resource/providers", token=admin, body={
    "name": f"{PROVIDER_PREFIX}-{suffix}", "provider_type": "dcimcloud",
    "api_endpoint": "http://p5-mock:9099", "status": 1, "des": "密码链路 e2e 自建（跑完清理）",
    "credentials": {"api_key": "e2e", "api_secret": "mock", "account_type": "admin"}})
provider_id = dig(obj, "id")
check("自建 dcimcloud 渠道（指向 p5-mock）", st == 200 and bool(provider_id), f"HTTP {st} {obj if st != 200 else ''}")
if not provider_id:
    cleanup_category(category_id)
    sys.exit(1)

st, obj = req("POST", "/admin/product/products", token=admin, body={
    "code": f"e2e-pwd-{suffix}", "name": f"{PRODUCT_PREFIX}-{suffix}",
    "product_type": "cloud_host", "price_model": "fixed", "price": 1.0, "cost_price": 0.0,
    "source_mode": "self", "source_provider_id": provider_id, "stock": -1, "status": 1,
    "category_id": category_id,
    "config_options": json.dumps({"rootpass": "e2e-rootpass-9xK"}, ensure_ascii=False),
    "description": "密码链路 e2e 自建（跑完清理）"})
self_product = dig(obj, "id")
check("自建 self 商品（config_options 带 rootpass）", st == 200 and bool(self_product), f"HTTP {st} {obj if st != 200 else ''}")
if not self_product:
    req("DELETE", f"/admin/resource/providers/{provider_id}", token=admin)
    cleanup_category(category_id)
    sys.exit(1)

u = f"pwd{suffix}"
req("POST", "/uc/auth/register", body={"username": u, "password": pwd, "email": f"{u}@example.com"})
st, obj = req("POST", "/uc/auth/login", body={"username": u, "password": pwd})
tok, uid = dig(obj, "token"), dig(obj, "id")
check("用户注册并登录", st == 200 and bool(tok), f"HTTP {st}")
st, obj = req("POST", "/admin/finance/transactions/adjust", token=admin, body={
    "user_id": uid, "type": "adjust", "direction": 1, "amount": 500,
    "biz_key": f"pwd-topup-{suffix}", "remark": "密码链路验收充值"})
check("充值", st == 200, f"HTTP {st} {obj}")

st, obj = req("POST", "/uc/orders", token=tok, body={"product_id": self_product, "quantity": 1}, timeout=30)
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

    # 6) 重置密码：新密码即刻成为权威值，「查看密码」必须返回新值而非旧值。
    new_pwd = "NewPass1234X"
    st, obj = req("POST", f"/admin/instances/{inst_id}/reset-password", token=admin, body={"password": new_pwd})
    check("重置密码接口返回成功", st == 200, f"HTTP {st} {obj if st != 200 else ''}")
    stored2 = sql(f"SELECT credential_password FROM instances WHERE id={inst_id}")
    check("重置后 instances.credential_password 已覆盖（不再是旧密文）",
          stored2.startswith("enc:v1:") and stored2 != stored, f"值={stored2[:24]}…")
    st, obj = req("POST", f"/admin/instances/{inst_id}/credential", token=admin)
    revealed2 = dig(obj, "password")
    check("重置后「查看密码」== 新密码（留存同步覆盖）", revealed2 == new_pwd,
          f"reveal={revealed2!r}")

    # 7) 重装系统：平台新签发初始凭据，一次性返回并同步覆盖留存。
    st, obj = req("POST", f"/admin/instances/{inst_id}/reinstall", token=admin,
                  body={"os": "62", "reason": "e2e 重装"})
    reinstall_pwd = dig(obj, "password")
    check("重装接口返回成功且一次性下发新凭据", st == 200 and bool(reinstall_pwd),
          f"HTTP {st} {obj if st != 200 else ''}")
    if reinstall_pwd:
        stored3 = sql(f"SELECT credential_password FROM instances WHERE id={inst_id}")
        check("重装后 instances.credential_password 已覆盖为新凭据",
              stored3.startswith("enc:v1:") and stored3 != stored2, f"值={stored3[:24]}…")
        st, obj = req("POST", f"/admin/instances/{inst_id}/credential", token=admin)
        revealed3 = dig(obj, "password")
        check("重装后「查看密码」== 平台新签发的凭据", revealed3 == reinstall_pwd,
              f"reveal={revealed3!r} reinstall={reinstall_pwd!r}")

# 清理自建资源：先删商品（渠道删除守卫要求无关联商品），再删渠道与分类。
# 渠道软删不报错即视为已删（不额外校验），残留由演示库维护者处置。
if self_product:
    st, obj = req("DELETE", f"/admin/product/products/{self_product}", token=admin)
    if st != 200:
        print(f"  [WARN] 自建商品 {self_product} 清理失败（HTTP {st}）：{obj}")
if provider_id:
    st, obj = req("DELETE", f"/admin/resource/providers/{provider_id}", token=admin)
    if st != 200:
        print(f"  [WARN] 自建渠道 {provider_id} 清理失败（HTTP {st}）：{obj}")
cleanup_category(category_id)
print("已清理自建渠道、商品与分类")

print()
if FAILURES:
    print(f"FAILED: {len(FAILURES)} 项 -> {FAILURES}")
    sys.exit(1)
print("ALL PASS")
