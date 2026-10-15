#!/usr/bin/env python3
"""前端类名孤儿检查：模板里用了、但全仓没有任何 CSS 定义的类名。

为什么需要这条门禁
------------------
本项目已**三次**交付过"模板写了类名、样式却从来没定义"的缺陷，症状都是
用户看到的裸样式（无边框、无间距、无布局），而且因为 Vue 不会报错、
typecheck 也不管 class，构建与测试全绿：

  1. `.state-spinning` / `.state-spin-icon` —— 状态动画引用了类名，
     但 `frontend-admin` 全仓没有定义，导致"过渡态转圈动画"一直没生效；
  2. `.kind-nav*` —— 产品线导航（全部实例 / 云主机 / 预埋类型）整块无样式，
     运营看到的是裸按钮和裸计数（用户实测反馈："这个按钮现在存在问题，
     就是没有样式，显示原始空间"）；
  3. `.page-header--actions` —— 详情页头改成动作条后，修饰类没写，
     按钮组被 `space-between` 推到左边、右侧留一大片空白。

共同点：**类名与样式分属两次提交、又没有机械校验**。本脚本把它变成机械检查。

判定口径
--------
- 只扫 static class（`class="a b"`）与 `:class` 里的**字符串字面量**
  （动态计算出来的类名无法静态判定，跳过并计数提示，不算违例）。
- TDesign 自带类（`t-` 前缀）与 Vue 过渡类（`v-` 前缀）天然没有本地定义，豁免。
- "有定义"的范围包括：模块 CSS、全局 CSS、任意 `.vue` 组件的 `<style>` 块
  （跨文件复用是允许的，一处定义即可）。
- 组件内联样式（`:style`）不算定义 —— 本检查针对样式类。

用法
----
    python3 scripts/check_frontend_class_orphans.py            # 检查（CI 门禁）
    python3 scripts/check_frontend_class_orphans.py --selfcheck # 逻辑自测

棘轮模式（与 check_structure.sh 一致）
------------------------------------
存量孤儿列入 `scripts/frontend_class_orphans_baseline.txt`（每行 `前端/相对路径: 类名`，
`#` 开头为注释）。基线内的不 fail 但会打印，提示"修复后请从基线移除"；
**基线之外的新增孤儿直接 fail**。这样门禁今天就能生效（拦住新缺陷），
又不要求一次性还清历史债。

退出码：0 通过；1 有基线外的新增孤儿（或自测失败）。
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

# 前端工程根目录（含各自 src）。
PROJECT_ROOT = Path(__file__).resolve().parent.parent
FRONTENDS = ["frontend-admin", "frontend-user", "frontend-site"]

# 存量孤儿基线（棘轮）：只降不升。修复后请从基线文件里删掉对应行。
BASELINE_FILE = Path(__file__).resolve().parent / "frontend_class_orphans_baseline.txt"

# 类名豁免：这些前缀由框架/组件库提供，本地本就不该有定义。
EXEMPT_PREFIXES = ("t-", "v-", "vue-", "el-")

# 类名里可能出现的、需要从模板字符串中剔除的插值痕迹。
TEMPLATE_INTERP = re.compile(r"\{\{.*?\}\}", re.S)


def strip_style_and_script(src: str) -> str:
    """去掉 <style> / <script> 段，只留标记部分。

    为什么要专门做这一步：前期用非贪婪的 `<template>(.*?)</template>` 想取模板段，
    但 Vue 的 `<template #slot>` 子标签到处都是，非贪婪匹配在**第一个子 template
    闭合处**就停了 —— 实际只扫到模板开头几十个字符，几乎整页被静默跳过
    （实测：注入一个显式孤儿类名，检查器毫无反应 = 假阴性）。
    改为"排除法"：去掉 style/script，剩下的就是标记，不再依赖 template 嵌套。
    """
    out = re.sub(r"<style[^>]*>.*?</style>", "", src, flags=re.S)
    out = re.sub(r"<script[^>]*>.*?</script>", "", out, flags=re.S)
    return out


def collect_template_classes(src: str) -> dict[str, set[str]]:
    """抽取标记段里的静态类名 → 出现位置集合。"""
    found: dict[str, set[str]] = {}
    tpl = strip_style_and_script(src)
    if tpl:
        # 1) 静态 class="a b c"
        #    用 (?<!:) 排除 :class —— 否则 :class="{...}" 会被当成静态类名，
        #    把 "'dyn-style':" "flag" 这类表达式片段误报为孤儿（自测实测）。
        for cm in re.finditer(r'(?<!:)\bclass="([^"]*)"', tpl):
            for cls in cm.group(1).split():
                cls = cls.strip()
                if not cls or "{" in cls or "}" in cls or "$" in cls:
                    continue
                found.setdefault(cls, set()).add("class")
        # 2) :class 表达式里的类名。
        #
        # 难点：同一个表达式里既有类名也有**比较值**，两者都是字符串字面量：
        #   :class="{ 'os-icon--lg': size === 'lg' }"
        #            ^^^^^^^^^^^^^^ 类名        ^^^^ 比较值（不是类名）
        # 前版本把比较值也当类名，导致 77 处误报（.danger/.failed/.lg/.plain…），
        # 误报率一高这条门禁就没人敢用了。判据：字面量紧跟在比较运算符
        # （=== / !== / == / !=）之后的，是取值而非类名。
        for cm in re.finditer(r":class=\"([^\"]*)\"", tpl):
            expr = cm.group(1)
            for lm in re.finditer(r"'([^']*)'|\"([^\"]*)\"", expr):
                raw = lm.group(1) or lm.group(2)
                if not raw:
                    continue
                # 字面量里含冒号 → 对象字面量的键值片段，跳过。
                if ":" in raw:
                    continue
                prefix = expr[: lm.start()].rstrip()
                if re.search(r"(===|!==|==|!=)$", prefix):
                    continue  # 比较值
                for cls in raw.split():
                    cls = cls.strip()
                    if not cls or "{" in cls or "}" in cls:
                        continue
                    found.setdefault(cls, set()).add(":class")
    return found


def collect_defined_classes(css_texts: list[str]) -> set[str]:
    """抽取 CSS 里定义过的类名（含属性选择器等一切 `.name` 形态）。"""
    defined: set[str] = set()
    for text in css_texts:
        for m in re.finditer(r"\.(-?[A-Za-z_][A-Za-z0-9_-]*)", text):
            defined.add(m.group(1))
    return defined


def load_sources(frontend: Path) -> tuple[list[Path], set[str], dict[Path, set[str]]]:
    """返回（.vue 文件, 全局已定义类名, 每个组件自己的已定义类名）。

    **scoped 与非 scoped 必须分开算**（这一点最初做错过，导致漏报）：
    `<style scoped>` 的规则会被 Vue 编译成 `.foo[data-v-xxx]`，只匹配**本组件**
    模板里的元素；写在 index.vue 的 scoped 样式，对 detail.vue 完全无效。

    实际后果：用户端 `云主机详情` 的过渡态图标用了 `state-spin-icon`，
    而该类只定义在同级 `index.vue` 的 scoped 块里 —— 检查器当时按"任意文件
    定义即可"判定，于是漏报，客户侧的转圈动画一直没生效。
    """
    vue_files = [p for p in frontend.rglob("*.vue") if "node_modules" not in p.parts and "dist" not in p.parts]

    # 全局：独立 .css 文件 + 组件里**不带 scoped** 的 style 块。
    global_texts: list[str] = []
    for p in frontend.rglob("*.css"):
        if "node_modules" in p.parts or "dist" in p.parts:
            continue
        global_texts.append(p.read_text(encoding="utf-8", errors="replace"))

    local: dict[Path, set[str]] = {}
    for p in vue_files:
        src = p.read_text(encoding="utf-8", errors="replace")
        for sm in re.finditer(r"<style([^>]*)>(.*?)</style>", src, re.S):
            attrs, body = sm.group(1), sm.group(2)
            if "scoped" in attrs:
                local.setdefault(p, set()).update(collect_defined_classes([body]))
            else:
                global_texts.append(body)

    return vue_files, collect_defined_classes(global_texts), local


def check_frontend(frontend: Path) -> list[tuple[Path, str, str]]:
    """返回该前端工程的孤儿类名列表：[(文件, 类名, 来源)]。

    某类名算"已定义"，当且仅当它出现在全局样式里，或出现在**本文件自己**的
    scoped 样式里。
    """
    if not frontend.is_dir():
        return []
    vue_files, global_defined, local_defined = load_sources(frontend)

    orphans: list[tuple[Path, str, str]] = []
    for p in vue_files:
        src = p.read_text(encoding="utf-8", errors="replace")
        src_no_interp = TEMPLATE_INTERP.sub("", src)
        own = local_defined.get(p, set())
        for cls, origins in collect_template_classes(src_no_interp).items():
            if cls.startswith(EXEMPT_PREFIXES):
                continue
            if cls in global_defined or cls in own:
                continue
            origin = "class" if "class" in origins else ":class"
            orphans.append((p, cls, origin))
    return sorted(orphans, key=lambda t: (str(t[0]), t[1]))


def load_baseline() -> set[str]:
    """读存量基线（`前端/路径: 类名`），忽略空行与注释。"""
    if not BASELINE_FILE.is_file():
        return set()
    entries: set[str] = set()
    for line in BASELINE_FILE.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if line and not line.startswith("#"):
            entries.add(line)
    return entries


def baseline_key(name: str, path: Path, cls: str) -> str:
    """基线条目的规范形态：`前端/相对路径: 类名`（与文件里的写法一致）。"""
    rel = path.relative_to(PROJECT_ROOT / name)
    return f"{name}/{rel}: {cls}"


def run_selfcheck() -> int:
    """自测：构造正反例，确认检查逻辑既不漏报也不误报。"""
    import tempfile

    with tempfile.TemporaryDirectory() as td:
        fe = Path(td) / "frontend-admin"
        (fe / "src").mkdir(parents=True)
        (fe / "src" / "styles").mkdir(parents=True)

        # 正例 1：有定义的类 → 不应报
        (fe / "src" / "ok.vue").write_text(
            '<template><div class="has-style t-card">x</div></template>', encoding="utf-8"
        )
        # 正例 2：动态类名里带定义的类 → 不应报
        (fe / "src" / "ok2.vue").write_text(
            '<template><div :class="{ \'dyn-style\': flag }">x</div></template>', encoding="utf-8"
        )
        (fe / "src" / "styles" / "a.css").write_text(
            ".has-style { color: red; }\n.dyn-style { color: blue; }\n", encoding="utf-8"
        )
        orphans = check_frontend(fe)
        if orphans:
            print(f"自测失败：有定义的类被误报 {orphans}", file=sys.stderr)
            return 1

        # 反例：新增一个没有样式的类 → 必须报
        (fe / "src" / "bad.vue").write_text(
            '<template><div class="orphan-class">x</div></template>', encoding="utf-8"
        )
        orphans = check_frontend(fe)
        names = {c for _, c, _ in orphans}
        if "orphan-class" not in names:
            print(f"自测失败：孤儿类名未被检出（实际 {sorted(names)}）", file=sys.stderr)
            return 1
        if "has-style" in names or "dyn-style" in names:
            print(f"自测失败：误报 {sorted(names)}", file=sys.stderr)
            return 1

        # 反例 2：:class 字面量里的孤儿 → 也必须报
        (fe / "src" / "bad2.vue").write_text(
            '<template><div :class="{ \'orphan-dyn\': flag }">x</div></template>', encoding="utf-8"
        )
        (fe / "src" / "bad.vue").unlink()
        names = {c for _, c, _ in check_frontend(fe)}
        if "orphan-dyn" not in names:
            print("自测失败：:class 字面量里的孤儿类名未被检出", file=sys.stderr)
            return 1
        (fe / "src" / "bad2.vue").unlink()

        # 反例 3：scoped 样式只在自身组件内生效 —— 别的组件用同名类必须报。
        #（最初漏报过：用户端详情页用了只定义在 index.vue scoped 块里的
        # .state-spin-icon，检查器按"任意文件定义即算"判，导致转圈动画静默失效。）
        (fe / "src" / "owner.vue").write_text(
            '<template><div class="scoped-only">x</div></template>\n'
            "<style scoped>.scoped-only { color: red; }</style>",
            encoding="utf-8",
        )
        names = {c for _, c, _ in check_frontend(fe)}
        if "scoped-only" in names:
            print("自测失败：本组件自己定义的 scoped 类被误报", file=sys.stderr)
            return 1

        (fe / "src" / "consumer.vue").write_text(
            '<template><div class="scoped-only">x</div></template>', encoding="utf-8"
        )
        names = {c for _, c, _ in check_frontend(fe)}
        if "scoped-only" not in names:
            print("自测失败：跨组件使用 scoped 类名未被检出（这正是漏报过的场景）", file=sys.stderr)
            return 1

        # 反例 4：孤儿出现在**嵌套 <template #slot> 之后**时也必须报。
        #（最初的模板抽取用非贪婪 `<template>(.*?)</template>`，会在第一个
        # 子 template 闭合处停止，导致 slot 之后的内容整段漏扫 —— 实测注入
        # 一个显式孤儿类名，检查器毫无反应 = 假阴性。改排除法后此例通过。）
        (fe / "src" / "consumer.vue").unlink()
        (fe / "src" / "nested.vue").write_text(
            "<template>\n"
            '  <div class="before-slot">a</div>\n'
            '  <template #extra><span>x</span></template>\n'
            '  <div class="after-slot-orphan">b</div>\n'
            "</template>",
            encoding="utf-8",
        )
        names = {c for _, c, _ in check_frontend(fe)}
        if "after-slot-orphan" not in names:
            print(
                "自测失败：嵌套 <template #slot> 之后的孤儿未被检出"
                "（非贪婪 template 抽取会整段漏扫）",
                file=sys.stderr,
            )
            return 1
        if "before-slot" not in names:
            print("自测失败：slot 之前的孤儿未被检出", file=sys.stderr)
            return 1
        (fe / "src" / "nested.vue").unlink()

        # 正例：把同一个类移进全局 .css 后，两个组件都不该再报。
        (fe / "src" / "styles" / "a.css").write_text(
            ".has-style { color: red; }\n.dyn-style { color: blue; }\n"
            ".scoped-only { color: green; }\n",
            encoding="utf-8",
        )
        names = {c for _, c, _ in check_frontend(fe)}
        if "scoped-only" in names:
            print("自测失败：类名提升到全局样式后仍被报为孤儿", file=sys.stderr)
            return 1

    print("自测通过：有定义的类不报，孤儿类名（含 :class 字面量）必报。")
    return 0


def main() -> int:
    ap = argparse.ArgumentParser(description="前端类名孤儿检查（模板用了但没有 CSS 定义）")
    ap.add_argument("--selfcheck", action="store_true", help="只跑逻辑自测")
    args = ap.parse_args()

    if args.selfcheck:
        return run_selfcheck()

    baseline = load_baseline()
    new_orphans: list[tuple[str, Path, str, str]] = []
    known: list[tuple[str, Path, str]] = []
    unused_baseline: list[str] = []
    seen_keys: set[str] = set()

    for name in FRONTENDS:
        for path, cls, origin in check_frontend(PROJECT_ROOT / name):
            key = baseline_key(name, path, cls)
            seen_keys.add(key)
            if key in baseline:
                known.append((name, path, cls))
            else:
                new_orphans.append((name, path, cls, origin))

    for entry in sorted(baseline - seen_keys):
        unused_baseline.append(entry)

    if known:
        print(f"存量孤儿 {len(known)} 处（基线内，不阻断；修复后请从基线移除）：")
        for name, path, cls in known:
            print(f"  {name}/{path.relative_to(PROJECT_ROOT / name)}: .{cls}")

    if unused_baseline:
        # 已修复但仍留在基线里 → 提示清理，避免基线虚高掩盖新问题。
        print(f"\n基线中有 {len(unused_baseline)} 条已不存在（应删除）：")
        for entry in unused_baseline:
            print(f"  {entry}")

    if new_orphans:
        print(f"\n[新增] {len(new_orphans)} 个类名在模板里使用，但当地无 CSS 定义：", file=sys.stderr)
        for name, path, cls, origin in new_orphans:
            rel = path.relative_to(PROJECT_ROOT)
            print(f"  {rel}: .{cls}  （来自 {origin}）", file=sys.stderr)
        print(
            "\n请补齐样式，或确认该类名确实不需要样式后从模板移除。\n"
            "背景：本项目已三次因\"类名与样式分属两次提交且无机械校验\"交付裸样式缺陷\n"
            "（.state-spinning 动画未生效 / .kind-nav 产品线导航无样式 / .page-header--actions）。\n"
            "scoped 提醒：<style scoped> 只对本组件生效，跨组件复用同名类名同样算孤儿。",
            file=sys.stderr,
        )
        return 1

    print(f"\n类名孤儿检查通过：无新增孤儿（存量 {len(known)} 处已登记基线）。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
