# e2e 验收脚本说明

本目录的 `e2e_*.py` 是**本地验收工具**，对运行中的开发环境（docker compose 容器 + 预置种子）做 HTTP 端到端断言。它们不在 CI 里跑——CI 只有 postgres service 容器，没有 docker-in-docker 的 mock 编排与演示种子；硬拉进 CI 会带来环境假失败。CI 仅保证这些脚本语法不腐坏（见 ci.yml 的 Python syntax check 步骤）。

统一入口 `smoke_local.sh` 按层级一键跑本地冒烟：

- `./scripts/smoke_local.sh --tier 1`：仅本地（referral + user_closure）
- `./scripts/smoke_local.sh --tier 2`：追加 p5-mock 依赖（password_chain + p5_fulfillment，要求容器已起）
- `./scripts/smoke_local.sh --tier 3`：追加开放平台（p6_open_api，要求 `OPEN_APP_ID`/`OPEN_APP_SECRET` 环境变量）
- 不带 `--tier` 时按环境自动探测：p5-mock 在跑就进 tier2，凭证齐全就进 tier3

## 运行前置总览

| 脚本 | 依赖容器/上游 | 种子数据 | 说明 |
| --- | --- | --- | --- |
| `e2e_referral.py` | 仅 backend + postgres | 自建商品（脚本内创建并清理） | 推广返现全链路，33 项断言 |
| `e2e_user_closure.py` | backend + postgres + 自建 uc-gw-mock（脚本内 docker run 自启动） | 无需外部上游 | 注销/恢复/留存期清理全链路；支付宝网关用本地 mock |
| `e2e_password_chain.py` | p5-mock 容器（9099）+ backend | 商品 183（self 链路绑渠道 16） | 密码密文落库、查看/重装回写 |
| `e2e_p5_fulfillment.py` | p5-mock 容器（9099）+ backend | 商品 1/渠道 16、17 | 双链路开通→续费→暂停→销毁→幂等重扫，37 项断言 |
| `e2e_p6_open_api.py` | backend（localhost:8080）+ p6-notify | openapp 应用凭证（--app-id/--app-secret 传入） | 开放平台签名/只读/代客下单/实例/回调；t65/t66 尚未实现 |

## 通用约定

- 都直连 `http://127.0.0.1:8080`（p6 用 `localhost:8080`），要求后端容器已在跑；
- 断言失败以非零退出码结束，输出 `失败项：` 清单；
- 涉及 mock 上游的脚本（password_chain/p5_fulfillment）要求 p5-mock 容器
  在 `backend_default` 网络并已加载最新 mock.py，否则上游断言全假失败；
- 脚本会把动过的环境配置**快照→强制→恢复**（user_closure 的核验自动放行开关、
  referral 的返现比率），跑完原样写回，演示库被运营改过也不影响验收。

## 维护提醒

- 改动下单/退款/注销/密码/开放平台链路后，对应脚本应在本地跑一遍；
- 脚本依赖的种子 ID（商品 183、渠道 16/17 等）是本地库的约定，
  换库验收前先核对种子存在性。

