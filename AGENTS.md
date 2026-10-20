---
name: "ai-diff-gate 项目规范"
description: "轻量 Go Demo：多模型扇出与文本对比"
---

## 项目定位

**ai-diff-gate** 是配置驱动的多模型对比 Demo：单 Prompt 并发扇出、行级文本 Diff、HTML 并排展示与 `POST /api/v1/compare` JSON API。非生产网关，不做 SSE / 计费 / OpenAI 透明代理。

## 核心目录

| 路径 | 职责 |
|------|------|
| `cmd/server/` | 进程入口、加载配置、注册 HTTP |
| `internal/adapter/` | OpenAI、Anthropic 等厂商 HTTP 适配 |
| `internal/provider/` | Provider 接口与归一化请求/响应类型 |
| `internal/fanout/` | 并发调用、超时、部分失败 continue |
| `internal/compare/` | 行级 unified diff |
| `internal/config/` | YAML 配置加载与 Provider 工厂 |
| `internal/handler/` | HTTP 路由与 HTML/JSON 响应 |
| `web/compare.html` | 对比页模板 |
| `docs/` | PRD、DESIGN、SPEC（权威契约） |

## 文档指针

- 产品与验收：`docs/PRD.md`
- 架构与模块：`docs/DESIGN.md`
- API、配置、用例：`docs/SPEC.md`
- 启动与配置：根目录 `README.md`

## 工程边界

- Go **1.27.1**；HTTP 使用标准库 `net/http`。
- Diff：`github.com/sergi/go-diff`；配置：YAML。
- API Key 优先 `api_key_env`；勿将真实 Key 提交仓库。
- 已拍板：部分失败 continue、单模型超时 60s、行级 diff、不忽略空白、省略 `model_ids` 则用配置全部模型。

## 验证入口

```bash
go build ./...
go run ./cmd/server
```

默认读取 `config.yaml`（可从 `config.example.yaml` 复制）；环境变量 `CONFIG_PATH` 可覆盖路径。

## 红线

- 不提交真实 API Key、`.env`、`.idea`。
- 缺少 `prompt` 时返回 400，不调用上游。
