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
"""
import json
import random
import sys
import urllib.error
import urllib.request

BASE = "http://127.0.0.1:8080/api/v1"
FAILURES = []

# 商品 1「标准云主机 2C4G」未绑定上游，开通即时完成（商品 2 绑定魔方财务，开通约 50s，
# 会超过服务端 10s write_timeout 导致客户端断连，不适合作为验收样本）。
PRODUCT_ID = 1


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


suffix = str(random.randint(100000, 999999))
ua, ub = f"refa{suffix}", f"refb{suffix}"
pwd = "Passw0rd!123"

print("== 0. 管理员登录 ==")
st, obj = req("POST", "/admin/auth/login", body={"username": "admin", "password": "123456"})
admin = dig(obj, "token")
check("管理员登录", st == 200 and bool(admin), f"HTTP {st} {obj if st != 200 else ''}")

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
    "product_id": PRODUCT_ID, "price": 100, "pay_mode": "balance"})
order1 = dig(obj, "id")
check("后台为 B 下单（余额支付+开通）", st == 200 and bool(order1), f"HTTP {st} {obj if st != 200 else ''}")

st, prof_a = req("GET", "/uc/referral/profile", token=tok_a)
bal, inc = money(dig(prof_a, "balance")), money(dig(prof_a, "total_income"))
check("首单按 10% 计提 → 余额 10", bal == 10.0 and inc == 10.0, f"balance={bal} income={inc}")

print("== 3. 幂等：重复开通同一订单 ==")
for _ in range(2):
    st, obj = req("POST", f"/admin/orders/{order1}/activate", token=admin, body={})
    check(f"重复开通返回成功（{st}）", st == 200, f"HTTP {st}")
st, prof_a = req("GET", "/uc/referral/profile", token=tok_a)
check("重复开通不重复计提", money(dig(prof_a, "balance")) == 10.0, f"balance={money(dig(prof_a,'balance'))}")

print("== 4. 后续订单按 5% 计提 ==")
st, obj = req("POST", f"/admin/users/{id_b}/orders", token=admin, body={
    "product_id": PRODUCT_ID, "price": 200, "pay_mode": "balance"})
check("第二单 200 创建成功", st == 200, f"HTTP {st} {obj if st != 200 else ''}")
st, prof_a = req("GET", "/uc/referral/profile", token=tok_a)
check("后续单按 5% 计提 → 余额 20", money(dig(prof_a, "balance")) == 20.0, f"balance={money(dig(prof_a,'balance'))}")

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
if FAILURES:
    print(f"结果：{len(FAILURES)} 项失败 → {FAILURES}")
    sys.exit(1)
print("结果：全部通过")
