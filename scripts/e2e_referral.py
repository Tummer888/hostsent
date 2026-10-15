#!/usr/bin/env python3
"""推广返现体系 HTTP 端到端验收（真实登录/下单/退款/提现/转入余额）。

覆盖：
  1. 邀请码注册绑定邀请关系
  2. 首单按首单比率计提 / 重复开通幂等
  3. 后续订单按后续比率计提
  4. 退款按比例冲减（重复审核不重复冲减）
  5. 提现申请冻结余额（最低提现额走 system_configs）
  6. 转入现金余额并落 wallet_transactions
  7. 旧代理域接口 404

环境假设与自愈（不依赖演示库的存量数据）：
  - 自建自营验收商品（self 链路即时开通，未绑上游、无规格、无选配），
    下单走 ManualAmount 手填价，比率断言不依赖商品定价；
  - 快照 → 强制 → 恢复 referral 分组配置（比率 10%/5%、最低提现 5 元），
    演示库被运营改过配置也不影响验收，跑完原样写回；
  - 用户行是注销/回收站/清除链路本来就约定的持久痕迹，脚本不再回滚。
"""
import json
import random
import sys
import time
import urllib.error
import urllib.request

BASE = "http://127.0.0.1:8080/api/v1"
FAILURES = []

# 自建验收商品的固定标签：清理时按名字前缀定位，不会误伤运营的商品。
PRODUCT_PREFIX = "e2e-referral-验收商品"
# 商品分类：建品现在强制必填（留空会落进「未分类」），脚本自建一个分类，
# 跑完在删完商品之后删除（分类下仍有商品时后端会拒绝删除）。
CATEGORY_PREFIX = "e2e-referral-分类"
# 下单手填价：算价管线优先级最高的 ManualAmount，比率断言只依赖本值。
PRICE_1, PRICE_2 = 100.0, 200.0
# 强制写入的返现配置（跑完原样恢复）：首单 10%、后续 5%、最低提现 5 元。
CONFIG_GROUP = "referral"
REFERRAL_RATE_KEYS = [
    "referral.enabled",
    "referral.first_order_rate",
    "referral.subsequent_rate",
    "referral.min_withdraw_amount",
]


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
        obj = json.loads(raw) if raw else None
    except json.JSONDecodeError:
        obj = raw
    return status, obj


def dig(obj, key):
    """深度优先查找嵌套响应中的 key（兼容 code/data 信封）。"""
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


def check(name, cond, detail=""):
    if not cond:
        FAILURES.append(name)
    print(f"  [{'PASS' if cond else 'FAIL'}] {name}{(' — ' + str(detail)) if detail else ''}")


def money(x):
    return round(float(x or 0), 2)


def profile_balance():
    """A 的返现画像余额/累计收入（异步计提落地前反复读取用）。"""
    st, prof_a = req("GET", "/uc/referral/profile", token=tok_a)
    if st != 200:
        return None, None
    return money(dig(prof_a, "balance")), money(dig(prof_a, "total_income"))


def wait_balance(want_bal, want_inc=None, secs=30):
    """等待返现计提落地：开通是异步工作池（约 2s 轮询）驱动的，断言前先收敛。"""
    deadline = time.time() + secs
    while time.time() < deadline:
        bal, inc = profile_balance()
        if bal == want_bal and (want_inc is None or inc == want_inc):
            return bal, inc
        time.sleep(1)
    return profile_balance()


def config_items(items):
    """构造批量 upsert 的配置项列表（逐键幂等，缺失则新建）。"""
    return [{
        "config_key": it["config_key"], "config_value": it["config_value"],
        "value_type": it["value_type"], "config_group": CONFIG_GROUP,
        "description": it["description"], "sort_order": it["sort_order"],
        "status": it["status"],
    } for it in items]


def upsert_configs(items):
    st, obj = req("POST", "/admin/system/configs/batch", token=admin,
                  body={"config_group": CONFIG_GROUP, "items": config_items(items)})
    return st, obj


def snapshot_configs():
    """快照 referral 分组里脚本要改动的 4 个键；快照失败返回 None 并禁用后续强制。"""
    st, obj = req("GET", f"/admin/system/configs/group/{CONFIG_GROUP}", token=admin)
    if st != 200:
        print(f"  [WARN] 快照 referral 配置失败（HTTP {st}），跳过强制/恢复：{obj}")
        return None
    rows = obj.get("data") or obj if isinstance(obj, dict) else obj
    if isinstance(rows, dict):
        rows = rows.get("items") or rows.get("data") or []
    if not isinstance(rows, list):
        return None
    saved = []
    for k in REFERRAL_RATE_KEYS:
        row = next((r for r in rows if isinstance(r, dict) and r.get("config_key") == k), None)
        if row is None:
            saved.append({"config_key": k, "config_value": "", "value_type": "string",
                          "description": "", "sort_order": 0, "status": "active", "exists": False})
            continue
        vt = row.get("value_type") or "string"
        if vt not in ("string", "bool", "int", "decimal", "json"):
            # 存量脏值（演示库历史写入的 number 等）：回写时归一为 decimal，
            # 否则恢复请求会因 value_type 校验失败而整体 400。
            vt = "decimal"
        saved.append({"config_key": k, "config_value": row.get("config_value") or "",
                      "value_type": vt, "description": row.get("description") or "",
                      "sort_order": row.get("sort_order") or 0,
                      "status": row.get("status") or "active", "exists": True})
    return saved


def restore_configs(saved):
    """按快照原样写回；快照为空（提前退出）跳过，键原本不存在则删除行。"""
    if not saved:
        return
    restore = [it for it in saved if it["exists"]]
    deleted = [it["config_key"] for it in saved if not it["exists"]]
    if restore:
        st, obj = upsert_configs(restore)
        if st != 200:
            print(f"  [WARN] referral 配置恢复失败（HTTP {st}）：{obj}")
            return
    for k in deleted:
        st, obj = req("GET", f"/admin/system/configs/{k}", token=admin)
        cid = dig(obj, "id")
        if st == 200 and cid:
            req("DELETE", f"/admin/system/configs/{cid}", token=admin)
    print(f"  referral 配置已恢复（更新 {len(restore)} 键，删除 {len(deleted)} 键）")


def cleanup_product(pid):
    """删除自建验收商品；已被实例引用（占位行）时放弃并提示，交由运营处置。"""
    if not pid:
        return
    st, obj = req("DELETE", f"/admin/product/products/{pid}", token=admin)
    if st != 200:
        print(f"  [WARN] 验收商品 {pid} 清理失败（HTTP {st}）：{obj}")


def cleanup_categories():
    """删除自建分类：必须在删完商品之后调用（分类下仍有商品时后端拒绝删除）。"""
    st, obj = req("GET", "/admin/product/categories", token=admin)
    if st != 200:
        return

    def walk(nodes):
        for node in nodes or []:
            if isinstance(node, dict):
                yield node
                yield from walk(node.get("children"))

    items = obj.get("data") or {}
    if isinstance(items, dict):
        items = items.get("items") or []
    for node in walk(items if isinstance(items, list) else []):
        if str(node.get("name") or "").startswith(CATEGORY_PREFIX):
            cid = node.get("id")
            st2, obj2 = req("DELETE", f"/admin/product/categories/{cid}", token=admin)
            if st2 != 200:
                print(f"  [WARN] 自建分类 {cid} 清理失败（HTTP {st2}）：{obj2}")


def cleanup_products():
    st, obj = req("GET", "/admin/product/products?page=1&page_size=200", token=admin)
    if st != 200:
        return
    rows = obj.get("data") or obj if isinstance(obj, dict) else obj
    if isinstance(rows, dict):
        rows = rows.get("items") or rows.get("list") or []
    if not isinstance(rows, list):
        return
    for p in rows:
        if isinstance(p, dict) and str(p.get("name") or "").startswith(PRODUCT_PREFIX):
            cleanup_product(p.get("id"))


suffix = str(random.randint(100000, 999999))
ua, ub = f"refa{suffix}", f"refb{suffix}"
pwd = "Passw0rd!123"
saved_configs = None
admin = None

# 自建验收商品：自营链路（source_mode=self）、未绑上游、无规格——下单即时本地开通，
# 不会触发上游开通请求。手填价由下单请求给定，因此商品定价任意、断言零依赖。
PRODUCT_CODE = f"e2e-referral-{suffix}"
PRODUCT_NAME = f"{PRODUCT_PREFIX}-{suffix}"

print("== 0. 管理员登录 ==")
st, obj = req("POST", "/admin/auth/login", body={"username": "admin", "password": "123456"})
admin = dig(obj, "token")
check("管理员登录", st == 200 and bool(admin), f"HTTP {st} {obj if st != 200 else ''}")
if not admin:
    sys.exit(1)

print("== 0.1 快照返现配置 ==")
saved_configs = snapshot_configs()
if saved_configs is not None:
    print(f"  已快照 referral 分组 {len(saved_configs)} 键")

print("== 0.2 强制返现配置 ==")
if saved_configs is not None:
    st, obj = upsert_configs([
        {"config_key": "referral.enabled", "config_value": "true", "value_type": "bool",
         "description": "e2e-referral 验收强制值（跑完恢复）", "sort_order": 1, "status": "active"},
        {"config_key": "referral.first_order_rate", "config_value": "0.10", "value_type": "decimal",
         "description": "e2e-referral 验收强制值（跑完恢复）", "sort_order": 1, "status": "active"},
        {"config_key": "referral.subsequent_rate", "config_value": "0.05", "value_type": "decimal",
         "description": "e2e-referral 验收强制值（跑完恢复）", "sort_order": 1, "status": "active"},
        {"config_key": "referral.min_withdraw_amount", "config_value": "5", "value_type": "decimal",
         "description": "e2e-referral 验收强制值（跑完恢复）", "sort_order": 1, "status": "active"},
    ])
    check("强制返现比率 10%/5% 与最低提现 5 元", st == 200, f"HTTP {st} {obj if st != 200 else ''}")
    if st != 200:
        sys.exit(1)

print("== 0.3 自建分类 + 自营验收商品 ==")
st, obj = req("POST", "/admin/product/categories", token=admin, body={
    "parent_id": 0, "name": f"{CATEGORY_PREFIX}-{suffix}", "sort_order": 0, "status": 1})
CATEGORY_ID = dig(obj, "id")
check("自建商品分类（供验收商品归属）", st == 200 and bool(CATEGORY_ID), f"HTTP {st} {obj if st != 200 else ''}")
if not CATEGORY_ID:
    restore_configs(saved_configs)
    sys.exit(1)

st, obj = req("POST", "/admin/product/products", token=admin, body={
    "code": PRODUCT_CODE, "name": PRODUCT_NAME, "product_type": "cloud_host",
    "price_model": "fixed", "price": 1.0, "cost_price": 0.0,
    "source_mode": "self", "stock": -1, "status": 1,
    "category_id": CATEGORY_ID,
    "description": "e2e-referral 验收自建（跑完自动清理）"})
PRODUCT_ID = dig(obj, "id")
check("自建商品（self 链路，未绑上游）", st == 200 and bool(PRODUCT_ID), f"HTTP {st} {obj if st != 200 else ''}")
if not PRODUCT_ID:
    restore_configs(saved_configs)
    sys.exit(1)

print("== 1. 邀请码注册 ==")
st, obj = req("POST", "/uc/auth/register", body={
    "username": ua, "password": pwd, "email": f"{ua}@example.com"})
check("注册 A", st == 200, f"HTTP {st} {obj if st != 200 else ''}")
st, obj = req("POST", "/uc/auth/login", body={"username": ua, "password": pwd})
tok_a, id_a = dig(obj, "token"), dig(obj, "id")
check("A 登录取得令牌", st == 200 and bool(tok_a), f"HTTP {st}")

st, prof_a = req("GET", "/uc/referral/profile", token=tok_a)
code_a = dig(prof_a, "invite_code")
check("A 自动获得邀请码", bool(code_a), code_a)

st, obj = req("POST", "/uc/auth/register", body={
    "username": ub, "password": pwd, "email": f"{ub}@example.com", "invite_code": code_a})
check("B 携邀请码注册", st == 200, f"HTTP {st} {obj if st != 200 else ''}")
id_b = dig(obj, "id")
st, obj = req("POST", "/uc/auth/login", body={"username": ub, "password": pwd})
tok_b = dig(obj, "token")
check("B 登录取得令牌", st == 200 and bool(tok_b), f"HTTP {st}")

st, obj = req("GET", f"/admin/referral/invitees?inviter_user_id={id_a}", token=admin)
names = json.dumps(obj, ensure_ascii=False)
check("A 的邀请关系已绑定 B", st == 200 and ub in names, f"HTTP {st} {names[:200]}")

print("== 2. 充值 + 首单计提 ==")
st, obj = req("POST", "/admin/finance/transactions/adjust", token=admin, body={
    "user_id": id_b, "type": "adjust", "direction": 1, "amount": 1000,
    "biz_key": f"e2e-topup-{suffix}", "remark": "返现 e2e 充值"})
check("B 充值 1000", st == 200, f"HTTP {st} {obj if st != 200 else ''}")

st, obj = req("POST", f"/admin/users/{id_b}/orders", token=admin, body={
    "product_id": PRODUCT_ID, "price": PRICE_1, "pay_mode": "balance"})
order1 = dig(obj, "id")
check("后台为 B 下单（余额支付+开通）", st == 200 and bool(order1), f"HTTP {st} {obj if st != 200 else ''}")

# 开通→计提走异步工作池；等待余额收敛到 10 再断言，避免与 worker 竞速。
bal, inc = wait_balance(10.0, 10.0)
check("首单按 10% 计提 → 余额 10", bal == 10.0 and inc == 10.0, f"balance={bal} income={inc}")

print("== 3. 幂等：重复开通同一订单 ==")
for _ in range(2):
    st, obj = req("POST", f"/admin/orders/{order1}/activate", token=admin, body={})
    check(f"重复开通返回成功（{st}）", st == 200, f"HTTP {st}")
# 重复开通走同步 Activate 即返回，但同样留 2s 让计提幂等落地后再读余额。
time.sleep(2)
bal, _ = profile_balance()
check("重复开通不重复计提", bal == 10.0, f"balance={bal}")

print("== 4. 后续订单按 5% 计提 ==")
st, obj = req("POST", f"/admin/users/{id_b}/orders", token=admin, body={
    "product_id": PRODUCT_ID, "price": PRICE_2, "pay_mode": "balance"})
check("第二单 200 创建成功", st == 200, f"HTTP {st} {obj if st != 200 else ''}")
bal, _ = wait_balance(20.0)
check("后续单按 5% 计提 → 余额 20", bal == 20.0, f"balance={bal}")

print("== 5. 退款按比例冲减 ==")
st, obj = req("POST", f"/admin/orders/{order1}/refund", token=admin, body={"amount": 50, "reason": "e2e 部分退款"})
refund_id = dig(obj, "id")
check("发起退款 50", st == 200 and bool(refund_id), f"HTTP {st} {obj if st != 200 else ''}")
st, obj = req("POST", f"/admin/refunds/{refund_id}/approve", token=admin, body={"remark": "e2e 通过"})
check("退款审核通过", st == 200, f"HTTP {st} {obj if st != 200 else ''}")
st, prof_a = req("GET", "/uc/referral/profile", token=tok_a)
check("冲减 10×50/100=5 → 余额 15", money(dig(prof_a, "balance")) == 15.0, f"balance={money(dig(prof_a,'balance'))}")
req("POST", f"/admin/refunds/{refund_id}/approve", token=admin, body={"remark": "重复审核"})
st, prof_a = req("GET", "/uc/referral/profile", token=tok_a)
check("重复审核不重复冲减", money(dig(prof_a, "balance")) == 15.0, f"balance={money(dig(prof_a,'balance'))}")

print("== 6. 配置：最低提现额改为 5 ==")
# 最低提现额已在 0.2 强制为 5；此处重放 upsert 验证分组批量保存链路本身。
st, obj = req("POST", "/admin/system/configs/batch", token=admin, body={
    "config_group": "referral",
    "items": [{"config_key": "referral.min_withdraw_amount", "config_value": "5",
               "value_type": "decimal", "config_group": "referral",
               "description": "最低提现金额（元）", "sort_order": 4, "status": "active"}]})
check("批量保存 referral 分组配置", st == 200, f"HTTP {st} {obj if st != 200 else ''}")

print("== 7. 提现申请冻结余额 ==")
st, obj = req("POST", "/uc/referral/withdrawals", token=tok_a, body={
    "amount": 10, "channel": "alipay", "account": "e2e@example.com"})
wid = dig(obj, "id")
check("申请提现 10", st == 200 and bool(wid), f"HTTP {st} {obj if st != 200 else ''}")
st, prof_a = req("GET", "/uc/referral/profile", token=tok_a)
check("余额 15→5、冻结 10",
      money(dig(prof_a, "balance")) == 5.0 and money(dig(prof_a, "frozen")) == 10.0,
      f"balance={money(dig(prof_a,'balance'))} frozen={money(dig(prof_a,'frozen'))}")

print("== 8. 转入现金余额 ==")
st, obj = req("GET", f"/admin/finance/wallets/{id_a}", token=admin)
before = money(dig(obj, "balance"))
st, obj = req("POST", "/uc/referral/transfer", token=tok_a, body={"amount": 5})
check("转入余额 5", st == 200, f"HTTP {st} {obj if st != 200 else ''}")
st, prof_a = req("GET", "/uc/referral/profile", token=tok_a)
# total_out = 累计流出：退款冲减 5 + 转出 5 = 10（提现 10 仍冻结，未流出不计入）
check("返现余额 5→0、累计流出 5+5=10",
      money(dig(prof_a, "balance")) == 0.0 and money(dig(prof_a, "total_out")) == 10.0,
      f"balance={money(dig(prof_a,'balance'))} out={money(dig(prof_a,'total_out'))}")
st, obj = req("GET", f"/admin/finance/wallets/{id_a}", token=admin)
after = money(dig(obj, "balance"))
check("现金余额 +5", after - before == 5.0, f"before={before} after={after}")

print("== 9. 管理端返现接口 + 旧代理域下线 ==")
for path, name in [("/admin/referral/cashbacks", "返现台账"),
                   ("/admin/referral/withdrawals", "提现列表"),
                   ("/admin/referral/invitees", "邀请关系")]:
    st, obj = req("GET", path, token=admin)
    check(f"GET {path}（{name}）", st == 200, f"HTTP {st}")
st, _ = req("GET", "/admin/distribution/agents", token=admin)
check("旧分销接口已下线 404", st == 404, f"HTTP {st}")
st, _ = req("GET", "/uc/agent/profile", token=tok_a)
check("旧 UC 代理接口已下线 404", st == 404, f"HTTP {st}")

print()
print(f"样本：A={ua}(id {id_a}, 码 {code_a})  B={ub}(id {id_b})  订单1={order1}  提现={wid}")

# 清理自建验收商品与恢复返现配置：无论验收成败都执行，演示库原样归还。
cleanup_products()
cleanup_categories()
restore_configs(saved_configs)
print("已清理自建验收商品与分类，并恢复 referral 配置")

if FAILURES:
    print(f"结果：{len(FAILURES)} 项失败 → {FAILURES}")
    sys.exit(1)
print("结果：全部通过")
