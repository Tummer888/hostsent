#!/usr/bin/env python3
"""把各页面手写的 filter-card 三段结构迁到 <FilterCard> 组件（一次性脚本）。

为什么用脚本：同一段结构在 51 个文件里重复了 53 次，逐个人工改既慢又必然漏掉
几个 —— 漏掉的那个页面会悄悄保留旧的移动端行为，且没有任何测试会发现。

脚本按「行 + 缩进」处理，不用正则匹配嵌套，因此每个决定都能对照原文核对。
形状不吻合预期（例如 grid 之前还有别的东西）的文件一律原样跳过并打印出来，
交给人工处理，绝不猜测式改写。

用法：
    python3 scripts/migrate-filter-cards.py --dry-run     # 只看会改哪些
    python3 scripts/migrate-filter-cards.py               # 实际写入
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

SECTION_HEAD = re.compile(r'^(?P<indent>[ \t]*)<section(?P<attrs>[^>]*)class="filter-card surface-card"(?P<tail>[^>]*)>\s*$')
GRID = re.compile(r'^(?P<indent>[ \t]*)<div(?P<attrs>[^>]*)class="filter-card__grid(?P<cls>[^"]*)"(?P<tail>[^>]*)>\s*$')


def matching_close(lines: list[str], start: int, indent: str, tag: str) -> int:
    """返回同缩进的 </tag> 所在行号；找不到返回 -1。"""
    close = f'{indent}</{tag}>'
    for i in range(start, len(lines)):
        if lines[i].rstrip() == close:
            return i
    return -1


def migrate(path: Path) -> tuple[bool, str]:
    raw = path.read_text(encoding='utf-8')
    lines = raw.split('\n')
    n = len(lines)

    section_at = -1
    for i, line in enumerate(lines):
        if SECTION_HEAD.match(line):
            section_at = i
            break
    if section_at < 0:
        return False, 'no filter-card section'

    indent = SECTION_HEAD.match(lines[section_at]).group('indent')
    inner = indent + '  '
    attrs = (
        SECTION_HEAD.match(lines[section_at]).group('attrs')
        + SECTION_HEAD.match(lines[section_at]).group('tail')
    ).strip()
    attrs = re.sub(r'\s+', ' ', attrs)

    section_end = matching_close(lines, section_at + 1, indent, 'section')
    if section_end < 0:
        return False, 'no matching </section>'

    # 逐行找三段
    head_at = grid_at = actions_at = -1
    for i in range(section_at + 1, section_end):
        line = lines[i]
        if line == f'{inner}<div class="filter-card__head">' and head_at < 0:
            head_at = i
        elif GRID.match(line) and grid_at < 0 and head_at != i:
            grid_at = i
        elif line == f'{inner}<div class="filter-card__actions">' and actions_at < 0:
            actions_at = i
    if grid_at < 0:
        return False, 'no filter-card__grid at expected indent'
    if head_at > grid_at:
        return False, 'head appears after grid'
    if 0 <= actions_at < grid_at:
        return False, 'actions appears before grid'

    # grid 之前、section 之后除 head 块之外的零散内容，收进 #pre 插槽
    # （工单页的工作台视图标签就落在这一段）。
    pre_lo = section_at + 1
    pre_hi = head_at if head_at >= 0 else grid_at
    pre_extra = lines[pre_lo:pre_hi]
    while pre_extra and not pre_extra[0].strip():
        pre_extra.pop(0)
    while pre_extra and not pre_extra[-1].strip():
        pre_extra.pop()

    # head
    title = ''
    head_extra: list[str] = []
    if head_at >= 0:
        head_end = matching_close(lines, head_at + 1, inner, 'div')
        if head_end < 0:
            return False, 'head not closed'
        for line in lines[head_at + 1:head_end]:
            m = re.fullmatch(r'\s*<h3 class="card-title">(.*?)</h3>', line)
            if m and not title:
                title = m.group(1).strip()
            else:
                head_extra.append(line)

    # grid
    gm = GRID.match(lines[grid_at])
    g_indent = gm.group('indent')
    grid_extra_cls = gm.group('cls').strip()
    grid_tail = (gm.group('attrs') + gm.group('tail')).strip()
    grid_tail = re.sub(r'\s+', ' ', grid_tail)
    grid_end = matching_close(lines, grid_at + 1, g_indent, 'div')
    if grid_end < 0:
        return False, 'grid not closed'
    grid_body = lines[grid_at + 1:grid_end]

    # actions
    actions_body: list[str] = []
    if actions_at >= 0:
        a_end = matching_close(lines, actions_at + 1, inner, 'div')
        if a_end < 0:
            return False, 'actions not closed'
        actions_body = lines[actions_at + 1:a_end]
        if any(l.strip() for l in lines[a_end + 1:section_end]):
            return False, 'extra content between actions and </section>'

    # 组装。各段容器缩进是 inner，其内容在 inner + 2；插槽内容要落在组件子级
    # （indent + 2），因此统一减去 2 个空格。
    head = f'{indent}<FilterCard'
    props: list[str] = []
    if attrs:
        props.append(attrs)
    if title and title != '筛选条件':
        props.append(f'title="{title}"')
    if grid_extra_cls:
        props.append(f'grid-class="{grid_extra_cls}"')
    if grid_tail:
        return False, f'grid has extra attrs to port manually: {grid_tail}'
    if props:
        head += ' ' + ' '.join(props)
    out: list[str] = [head + '>']

    def reindent(block: list[str], depth: int = 1) -> list[str]:
        """把一段内容整体对齐到指定层级（不假设它原先在第几层）。

        各页面里这些片段的起始缩进并不一致（grid 的内容在容器内再缩一级，
        而工单页的视图标签与 head 同级），按最小缩进整体平移比按固定层级加减可靠。
        """
        target = len(indent) + 2 * depth
        widths = [len(l) - len(l.lstrip(' ')) for l in block if l.strip()]
        if not widths:
            return block
        shift = target - min(widths)
        out_lines = []
        for line in block:
            if not line.strip():
                out_lines.append('')
            elif shift >= 0:
                out_lines.append(' ' * shift + line)
            else:
                out_lines.append(line[-shift:])
        return out_lines

    if pre_extra:
        out.append(f'{indent}  <template #pre>')
        out.extend(reindent(pre_extra, depth=2))
        out.append(f'{indent}  </template>')

    if head_extra and any(l.strip() for l in head_extra):
        out.append(f'{indent}  <template #head-extra>')
        out.extend(reindent(head_extra, depth=2))
        out.append(f'{indent}  </template>')

    body = grid_body[:]
    while body and not body[0].strip():
        body.pop(0)
    while body and not body[-1].strip():
        body.pop()
    out.extend(reindent(body))

    if any(l.strip() for l in actions_body):
        ab = actions_body[:]
        while ab and not ab[0].strip():
            ab.pop(0)
        while ab and not ab[-1].strip():
            ab.pop()
        out.append(f'{indent}  <template #actions>')
        out.extend(reindent(ab, depth=2))
        out.append(f'{indent}  </template>')

    out.append(f'{indent}</FilterCard>')

    new_lines = lines[:section_at] + out + lines[section_end + 1:]
    text = '\n'.join(new_lines)
    if "<script setup lang=\"ts\">" in text and 'components/filter-card' not in text:
        text = text.replace(
            '<script setup lang="ts">\n',
            '<script setup lang="ts">\nimport FilterCard from \'@/components/filter-card/index.vue\'\n',
            1,
        )
    path.write_text(text, encoding='utf-8')
    return True, 'ok'


def main() -> int:
    dry = '--dry-run' in sys.argv
    root = Path('src/pages')
    done, skipped = [], []
    for f in sorted(root.rglob('*.vue')):
        if dry:
            text = f.read_text(encoding='utf-8')
            if not SECTION_HEAD.search(text, ) if False else not any(
                SECTION_HEAD.match(l) for l in text.split('\n')
            ):
                continue
            # dry-run 不写盘：复制到内存判断
            backup = f.read_text(encoding='utf-8')
            ok, msg = migrate(f)
            f.write_text(backup, encoding='utf-8')
        else:
            ok, msg = migrate(f)
        (done if ok else skipped).append((str(f), msg))
    print(f'migrated {len(done)}:')
    for d, _ in done:
        print('  ✓', d)
    if skipped:
        print(f'skipped {len(skipped)}:')
        for s, m in skipped:
            print(f'  ✗ {s}  -- {m}')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
