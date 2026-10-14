#!/usr/bin/env python3
"""前端请求契约门禁（批次四 D4b）：把两个前端的 api 调用与后端路由表对齐。

后端唯一事实来源与 openapi 生成器同款（internal/server/*.go 的路由注册，
见 gen_openapi.py 的 parse_routes），前端扫描 src/api/*.ts 里的
request/instance.<verb>(...) 调用。检查三项：

  1. 每个前端调用的 (method, path) 在后端路由表里必须存在
     （路径参数 ${id} 与 :id/{id} 归一化比对，参数名不同不算漂移）；
  2. 同一条路径两端声明的方法集合必须一致（前端用 GET、后端只有 POST 即漂移）；
  3. 脚本自身的提取能力自测（--selfcheck）：故意喂错能检出。

前端拼前缀规则（与两个 request 实例的 baseURL 一一对应）：
  admin  src/api/*.ts 的 request.*   → /api/v1/admin + path
  admin  src/api/public.ts 的 instance.* → /api/v1/public + path
  user   src/api/public.ts 的 instance.* → /api/v1/public + path
  user   其余 src/api/*.ts           → /api/v1 + path（路径自带 uc/auth/menus 前缀）

用法：python3 scripts/check_api_contract.py
"""
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
BACKEND = ROOT / "backend"
FRONTEND_ADMIN = ROOT / "frontend-admin" / "src" / "api"
FRONTEND_USER = ROOT / "frontend-user" / "src" / "api"
FRONTEND_SITE = ROOT / "frontend-site"

VERBS = ("get", "post", "put", "delete", "patch")


def load_backend_routes():
    """复用 openapi 生成器的路由解析（Go 路由注册是唯一事实来源）。"""
    sys.path.insert(0, str(BACKEND / "scripts"))
    import gen_openapi

    return gen_openapi.parse_routes()


# 前端两种调用形态分开提取（互斥：对象形态要求 { 后直到本行内出现 url:，
# 直呼形态要求括号后紧跟引号，注释里的 "request.get 的 params..." 文本
# 两边都不命中）：
#   对象：request.get<...>({ url: '/path', ... })（{ 与 url: 同处一行）
#   直呼：request.get<...>('/path') / instance.post('/path')
CALL_OBJ_RE = re.compile(
    r"(?:request|instance)\.(get|post|put|delete|patch)(?:<[^>]*>)?"
    r"\(\{\s*url:\s*([\"'`])([^\"'`]+)\2"
)
CALL_DIRECT_RE = re.compile(
    r"(?:request|instance)\.(get|post|put|delete|patch)(?:<[^>]*>)?"
    r"\(\s*([\"'`])([^\"'`]+)\2"
)

# frontend-site 的 server 层用 $fetch(`${publicConfig.apiBase}/api/v1/...`)。
# 动词只可能是 GET（公开内容代理），前缀里带运行时变量。
SITE_FETCH_RE = re.compile(
    r"\$fetch(?:<[^>]*>)?\(\s*`\$\{publicConfig\.apiBase\}(/api/v1/[^`]*)`"
)
# encodeURIComponent(...) 包裹的路径参数在模板串里是表达式，匹配到 ${kind}/${encodeURIComponent(slug)} 形态，
# 归一化时把 ${encodeURIComponent(x)} 整段当作 {param} 处理。
SITE_EXPR_PARAM_RE = re.compile(r"\$\{[^}]*encodeURIComponent[^}]*\}")


def extract_calls(api_dir: Path):
    """返回 [(file, verb, path)]，按文件行序。"""
    calls = []
    for f in sorted(api_dir.glob("*.ts")):
        src = f.read_text()
        seen = set()
        for m in CALL_OBJ_RE.finditer(src):
            key = (m.start(), m.group(1), m.group(3))
            if key not in seen:
                seen.add(key)
                calls.append((str(f.relative_to(ROOT)), m.group(1), m.group(3)))
        for m in CALL_DIRECT_RE.finditer(src):
            # 捕获组：1=verb 2=引号 3=path；对象形态已匹配过的位置不再重复计入
            if m.group(1) in VERBS and (m.start(), m.group(1), m.group(3)) not in seen:
                calls.append((str(f.relative_to(ROOT)), m.group(1), m.group(3)))
    return calls


def extract_site_calls(site_dir: Path):
    """返回 [(file, verb, path)]：frontend-site 的 server 层 $fetch 调用。"""
    calls = []
    for f in sorted(site_dir.rglob("*.ts")):
        if "node_modules" in str(f):
            continue
        src = f.read_text()
        for m in SITE_FETCH_RE.finditer(src):
            calls.append((str(f.relative_to(ROOT)), "get", m.group(1)))
    return calls


def normalize(path: str) -> str:
    """把前端模板参数归一化：${id} / ${data.user_id} → {id}。

    ${encodeURIComponent(x)} 这类函数调用形参（frontend-site 的 slug 段）
    也整体归一为 {param}。"""
    path = SITE_EXPR_PARAM_RE.sub("{param}", path)
    return re.sub(r"\$\{([\w.]+)\}", r"{\1}", path)


def route_matches(route_path: str, call_path: str, verb: str) -> bool:
    """后端路由与前端调用比对：形状一致且该路由声明了相同动词。"""
    if verb not in VERBS:
        return False
    # 把 :param 与 {param} 统一成 {}，比较纯形状。
    shape = lambda p: re.sub(r"[:{][\w.]+[}]?", "{}", p)
    return shape(normalize(route_path)) == shape(normalize(call_path))


def resolve_prefix(rel_file: str, path: str, project: str) -> str:
    """按前端实例的 baseURL 把相对 path 还原成全路径。"""
    if rel_file.endswith("public.ts"):
        return "/api/v1/public" + path
    if project == "admin":
        return "/api/v1/admin" + path
    # user 端的路径自带 /uc/...、/auth、/menus 前缀
    return "/api/v1" + path


def check(verbose: bool = False):
    routes = load_backend_routes()
    failures = []
    checked = 0
    for project, api_dir, prefix_rule in (
        ("admin", FRONTEND_ADMIN, "admin"),
        ("user", FRONTEND_USER, "user"),
    ):
        for rel_file, verb, path in extract_calls(api_dir):
            checked += 1
            full = resolve_prefix(rel_file, path, prefix_rule)
            # 形状一致的路由里，只要有一条声明了相同动词即算命中。
            candidates = [rp for rp in routes if route_matches(rp, full, verb)]
            if not any(verb in routes[rp] for rp in candidates):
                detail = (
                    f"（后端只有 {sorted({v for rp in candidates for v in routes[rp]})}）"
                    if candidates
                    else "在后端路由表里不存在"
                )
                failures.append(f"{rel_file}: {verb.upper()} {full} {detail}")
    # frontend-site 的 server 层（BFF 代理）：路径已带完整 /api/v1 前缀，动词为 get。
    for rel_file, verb, path in extract_site_calls(FRONTEND_SITE):
        checked += 1
        candidates = [rp for rp in routes if route_matches(rp, path, verb)]
        if not any(verb in routes[rp] for rp in candidates):
            detail = (
                f"（后端只有 {sorted({v for rp in candidates for v in routes[rp]})}）"
                if candidates
                else "在后端路由表里不存在"
            )
            failures.append(f"{rel_file}: {verb.upper()} {path} {detail}")
    if verbose:
        print(f"契约扫描：{checked} 次前端调用 / {len(routes)} 条后端路由")
    return failures, checked


def selfcheck():
    """提取/比对逻辑自测：故意喂错必须能检出（防检查退化）。"""
    shape = lambda p: re.sub(r"[:{][\w.]+[}]?", "{}", p)
    norm = lambda p: re.sub(r"\$\{([\w.]+)\}", r"{\1}", p)
    cases = [
        (("/api/v1/admin/orders/{id}", "/api/v1/admin/orders/${orderId}", "post"),
         True, "参数名不同应视为匹配"),
        (("/api/v1/admin/orders/batch-cancel", "/api/v1/admin/orders/batch-cancel", "post"),
         True, "静态路径应精确匹配"),
        (("/api/v1/admin/orders", "/api/v1/admin/ordersx", "get"),
         False, "路径拼写漂移必须检出"),
        (("/api/v1/admin/orders/{id}", "/api/v1/admin/orders/{id}/x", "post"),
         False, "多余后缀必须检出"),
        (("/api/v1/admin/users/{id}/impersonate", "/api/v1/admin/users/${data.user_id}/impersonate", "post"),
         True, "${data.user_id} 这类对象属性形参必须能归一化"),
        (("/api/v1/public/articles/{kind}/{slug}", "/api/v1/public/articles/${kind}/${encodeURIComponent(slug)}", "get"),
         True, "frontend-site 的 encodeURIComponent 形参必须能归一化"),
    ]
    for (route, call, verb), want, why in cases:
        # route_matches 只比较形状；动词一致性在 check() 里比对
        got = shape(normalize(route)) == shape(normalize(call))
        if got != want:
            print(f"selfcheck 失败：{why}（{route} vs {call}）", file=sys.stderr)
            return 1
    # 动词维度自测：check() 的核心判断「候选路由里是否声明了相同动词」。
    fake_routes = {"/api/v1/x/{id}": {"get"}}
    from pathlib import Path

    def fake_check(verb):
        full = f"/api/v1/x/${{id}}"
        candidates = [rp for rp in fake_routes if route_matches(rp, full, verb)]
        return any(verb in fake_routes[rp] for rp in candidates)

    if fake_check("post") or not fake_check("get"):
        print("selfcheck 失败：方法漂移判断退化（POST 应检出、GET 应通过）", file=sys.stderr)
        return 1
    print("fixture 自测通过（路径形状 + 方法漂移均可检出）")
    return 0


def main():
    if "--selfcheck" in sys.argv:
        return selfcheck()
    failures, checked = check(verbose=True)
    if failures:
        print(f"契约门禁失败：{len(failures)} 处前后端漂移", file=sys.stderr)
        for f in failures:
            print("  " + f, file=sys.stderr)
        return 1
    print(f"契约门禁通过：{checked} 次前端调用全部落在后端路由表")
    return 0


if __name__ == "__main__":
    sys.exit(main())
