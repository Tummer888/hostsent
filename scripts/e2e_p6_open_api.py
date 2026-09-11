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
#   - 应用已用 cmd/openapp 创建（owner_user_id 指向真实用户）。
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
import sys
import time
import urllib.parse
import urllib.request
import uuid

BASE = "http://localhost:8080"

PASS = 0
FAIL = 0
FAILED = []


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

def run_t62(c: OpenClient):
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
    target = next((it for it in items if it.get("id") == 1), None)
    check("商品 1（在售）可见", target is not None, f"{items[:2]}")

    status, resp = c.request("GET", "/open/v1/products/1")
    detail = resp.get("data") or {}
    check("商品详情返回 0", code_of(resp) == 0 and detail.get("id") == 1, f"{code_of(resp)}")

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

    # 询价（D3：owner 组折扣 0.85；policy item 绑定商品 1）
    status, resp = c.request("POST", "/open/v1/quote", body={"product_id": 1, "quantity": 1})
    quote = resp.get("data") or {}
    check("quote 返回 0 且金额>0", code_of(resp) == 0 and quote.get("final_amount", 0) > 0, f"{resp}")
    original = quote.get("original_amount", 0)
    final = quote.get("final_amount", 0)
    expected = round(original * 0.85, 2)
    check(f"quote 实付=原价×0.85（{final} == {expected}）", abs(final - expected) < 0.011, f"{quote}")
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

    if only in ("all", "t61"):
        run_t61(client)
    if only in ("all", "t62"):
        run_t62(client)
        if args.readonly_app_id and args.readonly_app_secret:
            run_t62_scope_denied(OpenClient(args.readonly_app_id, args.readonly_app_secret))

    print(f"\n结果：PASS={PASS} FAIL={FAIL}")
    if FAILED:
        print("失败项：")
        for name in FAILED:
            print(f"  - {name}")
    sys.exit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
