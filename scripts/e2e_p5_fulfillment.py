#!/usr/bin/env python3
"""P5 履约双通道 HTTP 端到端验收（双链路：下单→开通→续费→到期暂停）。

前置（脚本不负责创建，见 docs/实施计划/17 §P5 交接说明）：
  - mock 上游容器 p5-mock 运行在 backend_default 网络，端口 9099；
  - 渠道 16 = mofangyun（链路 B 自营平台），渠道 17 = mofangfinance（链路 A 上游转售）；
  - 商品 1（self，68/月，绑渠道 16）、商品 2（upstream，30/月，绑渠道 17 + 资源商品 52）。

断言（doc17 §7 P5 验收）：
  A. 两链路各跑通「下单→开通→续费→到期暂停」闭环；
  B. 重复扫描不重复调用上游（暂停计数恒为 1）；
  C. 异步开通：下单接口立即返回 paid + provision_status=pending，工作池随后置 active；
  D. 链路 A 续费以上游返回账期为权威（sync_state=upstream_ok，upstream_order_id 落库）；
     链路 B 无续费能力时本地顺延（sync_state=local_only）且不误报失败。
"""
import json
import random
import subprocess
import sys
import time
import urllib.error
import urllib.request

BASE = "http://127.0.0.1:8080/api/v1"
MOCK = ["docker", "exec", "p5-mock", "cat", "/state/state.json"]
PSQL = ["docker", "exec", "backend-postgres-1", "psql", "-U", "hostsent", "-d", "hostsent", "-tAc"]
FAILURES = []

SELF_PRODUCT = 1    # 链路 B：自营，平台 = 渠道 16 魔方云 mock
UPSTREAM_PRODUCT = 2  # 链路 A：转售，上游 = 渠道 17 魔方财务 mock


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


def ledger():
    out = subprocess.run(MOCK, capture_output=True, text=True, check=True).stdout
    return json.loads(out or "{}")


def sql(query):
    out = subprocess.run(PSQL + [query], capture_output=True, text=True)
    if out.returncode != 0:
        raise RuntimeError(out.stderr.strip())
    return out.stdout.strip()


def poll_order(token, order_id, want="active", timeout=90):
    """轮询我的订单，返回 (status, provision_status)；超时返回最后状态。"""
    deadline = time.time() + timeout
    last = ("", "")
    while time.time() < deadline:
        st, obj = req("GET", "/uc/orders?page=1&page_size=50", token=token)
        items = (obj or {}).get("data", {}).get("items") if isinstance(obj, dict) else None
        if isinstance(items, list):
            for it in items:
                if it.get("id") == order_id:
                    last = (it.get("status", ""), it.get("provision_status", ""))
                    if it.get("status") == want:
                        return last
        time.sleep(1)
    return last


def instance_row(order_id):
    row = sql(f"SELECT id,source_mode,provider_id,sell_product_id,upstream_product_id,"
              f"provider_instance_id,lifecycle_stage,expire_at FROM instances WHERE order_id={order_id} LIMIT 1")
    if not row:
        return None
    parts = row.split("|")
    return {
        "id": int(parts[0]), "source_mode": parts[1], "provider_id": int(parts[2]),
        "sell_product_id": parts[3], "upstream_product_id": parts[4],
        "provider_instance_id": parts[5], "lifecycle_stage": parts[6], "expire_at": parts[7],
    }


def set_expire(inst_id, days_ago):
    sql(f"UPDATE instances SET expire_at = now() - interval '{days_ago} days', "
        f"lifecycle_stage = NULL WHERE id={inst_id}")


def scan(admin):
    st, obj = req("POST", "/admin/lifecycle/scan", token=admin, body={})
    return st, obj


def chain_headers(title):
    print(f"\n== {title} ==")


suffix = str(random.randint(100000, 999999))
pwd = "Passw0rd!123"

print("== 0. 准备：清理历史自营 test 实例的扫描干扰 + 管理员登录 ==")
# lc-test-02（id 10）到期时间落在暂停窗口内但 provider 指向真实上游，会把每轮扫描拖进真实网络调用；
# 标记为已暂停使其退出候选（仅测试库数据，不影响验收目标实例）。
sql("UPDATE instances SET lifecycle_stage='suspended' WHERE instance_id IN ('lc-test-01','lc-test-02')")
st, obj = req("POST", "/admin/auth/login", body={"username": "admin", "password": "123456"})
admin = dig(obj, "token")
check("管理员登录", st == 200 and bool(admin), f"HTTP {st} {obj if st != 200 else ''}")

users = {}
for label, product in [("B", SELF_PRODUCT), ("A", UPSTREAM_PRODUCT)]:
    u = f"p5{label.lower()}{suffix}"
    st, obj = req("POST", "/uc/auth/register", body={"username": u, "password": pwd, "email": f"{u}@example.com"})
    st, obj = req("POST", "/uc/auth/login", body={"username": u, "password": pwd})
    tok, uid = dig(obj, "token"), dig(obj, "id")
    check(f"用户 {label} 注册并登录", st == 200 and bool(tok) and bool(uid), f"HTTP {st} {obj if st != 200 else ''}")
    st, obj = req("POST", "/admin/finance/transactions/adjust", token=admin, body={
        "user_id": uid, "type": "adjust", "direction": 1, "amount": 1000,
        "biz_key": f"p5-topup-{label}-{suffix}", "remark": "P5 履约验收充值"})
    check(f"用户 {label} 充值 1000", st == 200, f"HTTP {st} {obj if st != 200 else ''}")
    users[label] = {"name": u, "id": uid, "token": tok, "product": product}

# ---------------------------------------------------------------------------
# 链路 B：自营（商品 1 → 渠道 16 魔方云 mock）
# ---------------------------------------------------------------------------
chain_headers("链路 B｜自营：下单 → 异步开通 → 续费 → 到期暂停")
ub = users["B"]
before = ledger()
created0 = before["cloud_creates"]

t0 = time.time()
st, obj = req("POST", "/uc/orders", token=ub["token"], body={"product_id": ub["product"], "quantity": 1}, timeout=30)
elapsed = time.time() - t0
order_b = dig(obj, "id")
st_pay, prov = dig(obj, "status"), dig(obj, "provision_status")
check("B 下单接口立即返回（<10s 写超时）", st == 200 and elapsed < 10, f"HTTP {st} 用时 {elapsed:.2f}s")
check("B 订单初始为 paid + provision_status=pending（异步开通 T5.1）",
      st_pay == "paid" and prov == "pending", f"status={st_pay} provision={prov}")

final_b, prov_final = poll_order(ub["token"], order_b, want="active")
check("B 工作池推进订单到 active", final_b == "active", f"最后 status={final_b} provision={prov_final}")
after_create = ledger()
check("B 开通恰好调用上游创建一次（cloud_creates +1）",
      after_create["cloud_creates"] - created0 == 1,
      f"cloud_creates {created0}→{after_create['cloud_creates']}")
check("B 开通未误触上游续费/暂停", after_create["renew"] == before["renew"] and after_create["mff_suspend"] == before["mff_suspend"])

inst_b = instance_row(order_b)
check("B 实例落库：链路 self、平台实例号、售出商品 1",
      inst_b is not None and inst_b["source_mode"] == "self" and inst_b["sell_product_id"] == "1"
      and inst_b["provider_instance_id"] != "" and inst_b["upstream_product_id"] in ("", "0"),
      inst_b)

# 到期暂停：把 expire_at 放进 suspended 窗口（grace 7 天 → (now-37d, now-7d]）
set_expire(inst_b["id"], 8)
susp0 = ledger()["mfy_suspend"]
scan(admin)
mid = ledger()
check("B 首次扫描触发一次平台暂停", mid["mfy_suspend"] - susp0 == 1, f"mfy_suspend {susp0}→{mid['mfy_suspend']}")
row = sql(f"SELECT lifecycle_stage FROM instances WHERE id={inst_b['id']}")
check("B 阶段落库为 suspended", row == "suspended", row)
scan(admin)
scan(admin)
now_led = ledger()
check("B 重复扫描不重复调用上游（暂停计数仍为 1）", now_led["mfy_suspend"] - susp0 == 1,
      f"mfy_suspend 增量 {now_led['mfy_suspend'] - susp0}")

# 续费：魔方云无独立续费接口 → 链路 B 本地顺延，不得整体失败
renew0 = ledger()
st, obj = req("POST", f"/admin/instances/{inst_b['id']}/renew", token=admin, body={"period_count": 1})
check("B 续费成功（缺上游续费能力时本地顺延）", st == 200, f"HTTP {st} {obj if st != 200 else ''}")
sync_state = sql(f"SELECT sync_state FROM instance_renewals WHERE instance_id={inst_b['id']} ORDER BY id DESC LIMIT 1")
check("B 续费 sync_state=local_only", sync_state == "local_only", sync_state)
new_exp = sql(f"SELECT expire_at FROM instances WHERE id={inst_b['id']}")
check("B 续费后到期时间推后到未来", new_exp > sql("SELECT now()"), new_exp)
check("B 续费未调用上游创建/财务接口", ledger()["cloud_creates"] == now_led["cloud_creates"]
      and ledger()["renew"] == renew0["renew"])

# 续费后回退 active：应先解除暂停，再落 active
uns0 = ledger()["mfy_unsuspend"]
scan(admin)
check("B 续费后扫描恢复实例（unsuspend 一次）", ledger()["mfy_unsuspend"] - uns0 == 1,
      f"mfy_unsuspend 增量 {ledger()['mfy_unsuspend'] - uns0}")
check("B 阶段回退 active", sql(f"SELECT lifecycle_stage FROM instances WHERE id={inst_b['id']}") == "active")
scan(admin)
check("B 回退后重复扫描不再调上游", ledger()["mfy_unsuspend"] - uns0 == 1)

# ---------------------------------------------------------------------------
# 链路 A：上游转售（商品 2 → 渠道 17 魔方财务 mock）
# ---------------------------------------------------------------------------
chain_headers("链路 A｜上游转售：下单 → 异步开通 → 续费 → 到期暂停")
ua = users["A"]
before = ledger()

st, obj = req("POST", "/uc/orders", token=ua["token"], body={"product_id": ua["product"], "quantity": 1}, timeout=30)
order_a = dig(obj, "id")
check("A 订单初始为 paid + provision_status=pending",
      st == 200 and dig(obj, "status") == "paid" and dig(obj, "provision_status") == "pending",
      f"HTTP {st} status={dig(obj, 'status')} provision={dig(obj, 'provision_status')}")

final_a, prov_final = poll_order(ua["token"], order_a, want="active", timeout=120)
check("A 工作池推进订单到 active", final_a == "active", f"最后 status={final_a} provision={prov_final}")
after_a = ledger()
check("A 开走上游财务下单流程（cart/add_to_shop + cart/settle 各一次）",
      after_a["cart_add"] - before["cart_add"] == 1 and after_a["cart_settle"] - before["cart_settle"] == 1,
      f"cart_add {after_a['cart_add'] - before['cart_add']} cart_settle {after_a['cart_settle'] - before['cart_settle']}")
check("A 开通未误触自营平台（cloud_creates 不变）",
      after_a["cloud_creates"] == before["cloud_creates"], f"cloud_creates={after_a['cloud_creates']}")

inst_a = instance_row(order_a)
check("A 实例落库：链路 upstream、上游商品 52、渠道 17、售出商品 2",
      inst_a is not None and inst_a["source_mode"] == "upstream" and inst_a["upstream_product_id"] == "52"
      and inst_a["provider_id"] == 17 and inst_a["sell_product_id"] == "2",
      inst_a)

# 到期暂停：走上游 /provision/default func=suspend
set_expire(inst_a["id"], 8)
susp0 = ledger()["mff_suspend"]
scan(admin)
check("A 首次扫描触发一次上游暂停", ledger()["mff_suspend"] - susp0 == 1,
      f"mff_suspend {susp0}→{ledger()['mff_suspend']}")
check("A 阶段落库为 suspended", sql(f"SELECT lifecycle_stage FROM instances WHERE id={inst_a['id']}") == "suspended")
scan(admin)
scan(admin)
check("A 重复扫描不重复调用上游（暂停计数仍为 1）", ledger()["mff_suspend"] - susp0 == 1,
      f"增量 {ledger()['mff_suspend'] - susp0}")

# 续费：链路 A 必须调上游，以上游 nextduedate 为权威账期
r0 = ledger()
st, obj = req("POST", f"/admin/instances/{inst_a['id']}/renew", token=admin, body={"period_count": 1})
check("A 续费成功", st == 200, f"HTTP {st} {obj if st != 200 else ''}")
check("A 续费调用上游 host/renew + apply_credit",
      ledger()["renew"] - r0["renew"] == 1 and ledger()["apply_credit"] - r0["apply_credit"] == 1,
      f"renew {ledger()['renew'] - r0['renew']} apply_credit {ledger()['apply_credit'] - r0['apply_credit']}")
row = sql(f"SELECT sync_state, upstream_order_id FROM instance_renewals WHERE instance_id={inst_a['id']} ORDER BY id DESC LIMIT 1")
sync_state, up_ref = (row.split("|") + ["", ""])[:2]
check("A 续费 sync_state=upstream_ok 且落上游账单号", sync_state == "upstream_ok" and up_ref != "",
      f"sync_state={sync_state} upstream_order_id={up_ref}")
exp_a = sql(f"SELECT expire_at FROM instances WHERE id={inst_a['id']}")
check("A 账期以上游返回的 nextduedate 为准（2027-01-01）", exp_a.startswith("2027-01-01"), exp_a)

uns0 = ledger()["mff_unsuspend"]
scan(admin)
check("A 续费后扫描恢复实例（上游 unsuspend 一次）", ledger()["mff_unsuspend"] - uns0 == 1,
      f"增量 {ledger()['mff_unsuspend'] - uns0}")
check("A 阶段回退 active", sql(f"SELECT lifecycle_stage FROM instances WHERE id={inst_a['id']}") == "active")

# 销毁（T5.3/T5.5）：链路 A 走上游 host/cancel，重复扫描不重复调用
chain_headers("销毁通道｜两链路各自调用上游终止接口且仅一次")
set_expire(inst_a["id"], 40)  # 超出 grace(7)+destroy_keep(30) → destroyed 窗口
d0 = ledger()["mff_delete"]
scan(admin)
check("A 首次扫描触发一次上游终止（host/cancel）", ledger()["mff_delete"] - d0 == 1,
      f"mff_delete {d0}→{ledger()['mff_delete']}")
check("A 阶段落库为 destroyed", sql(f"SELECT lifecycle_stage FROM instances WHERE id={inst_a['id']}") == "destroyed")
scan(admin)
check("A 重复扫描不重复调用上游终止", ledger()["mff_delete"] - d0 == 1)

set_expire(inst_b["id"], 40)
d0b = ledger()["mfy_delete"]
scan(admin)
check("B 首次扫描触发一次平台删除（DELETE /clouds/{id}）", ledger()["mfy_delete"] - d0b == 1,
      f"mfy_delete {d0b}→{ledger()['mfy_delete']}")
check("B 阶段落库为 destroyed", sql(f"SELECT lifecycle_stage FROM instances WHERE id={inst_b['id']}") == "destroyed")
scan(admin)
check("B 重复扫描不重复调用上游删除", ledger()["mfy_delete"] - d0b == 1)

print(f"\n样本：用户B={users['B']['name']}(id {users['B']['id']}) 订单 {order_b} 实例 {inst_b['id']}；"
      f"用户A={users['A']['name']}(id {users['A']['id']}) 订单 {order_a} 实例 {inst_a['id']}")
print("上游台账：", json.dumps({k: v for k, v in ledger().items() if k not in ("calls", "last_create")}, ensure_ascii=False))
if FAILURES:
    print(f"\n结果：{len(FAILURES)} 项失败 → {FAILURES}")
    sys.exit(1)
print("\n结果：全部通过")
