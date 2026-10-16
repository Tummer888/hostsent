#!/usr/bin/env python3
"""swag 生成后的 OpenAPI 3.1 文档补丁。

swag v2.0.0 的 OAS3 模式仅注册第一个 @securitydefinitions（上游缺陷），
开放平台 /open/v1 的签名鉴权需要在生成结果中补齐：

1. 向 components.securitySchemes 注入 OpenSignature 定义；
2. 将 /open/v1/** 操作的 security 替换为 OpenSignature（清除误标的 BearerAuth）。

脚本幂等，可重复执行。CI 门禁在 swag init 之后追加调用本脚本。
"""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DOC = ROOT / "docs" / "swagger.json"

OPEN_SIGNATURE = {
    "type": "apiKey",
    "in": "header",
    "name": "X-Signature",
    "description": (
        "开放平台签名鉴权：请求需同时携带 X-App-Id、X-Timestamp、"
        "X-Nonce 与本头，签名算法为 HMAC-SHA256(StringToSign, appSecret)。"
        "StringToSign = METHOD + PATH + SORTED_QUERY + SHA256(body) + timestamp + nonce。"
    ),
}


def main() -> int:
    if not DOC.exists():
        print(f"swagger.json 不存在: {DOC}", file=sys.stderr)
        return 1
    doc = json.loads(DOC.read_text())
    components = doc.setdefault("components", {})
    schemes = components.setdefault("securitySchemes", {})

    patched = False
    if "OpenSignature" not in schemes:
        schemes["OpenSignature"] = OPEN_SIGNATURE
        patched = True
    if "BearerAuth" not in schemes:
        schemes["BearerAuth"] = {
            "type": "http",
            "scheme": "bearer",
            "bearerFormat": "JWT",
            "description": "JWT Bearer 鉴权（管理端/用户中心）。",
        }
        patched = True

    n_ops = 0
    for path, item in doc.get("paths", {}).items():
        if not path.startswith("/open/v1/"):
            continue
        for method, op in item.items():
            if method not in ("get", "post", "put", "patch", "delete"):
                continue
            if op.get("security") != [{"OpenSignature": []}]:
                op["security"] = [{"OpenSignature": []}]
                n_ops += 1
                patched = True

    DOC.write_text(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
    print(f"patch_openapi: schemes={list(schemes)} open_ops_secured={n_ops}")
    return 0 if patched or n_ops == 0 else 0


if __name__ == "__main__":
    sys.exit(main())
