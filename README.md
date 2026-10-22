# ai-diff-gate

轻量 Go Demo：配置驱动接入多模型，对同一 Prompt 并发扇出，行级文本 Diff，HTML 并排展示与 JSON 对比 API。

## 功能

- OpenAI、Anthropic、DeepSeek 协议适配（可扩展 adapter）
- Fan-out 并发调用，单模型超时默认 60s，部分失败 continue
- 行级 unified diff（不忽略空白）
- `GET` / `POST /compare` 对比页，`POST /api/v1/compare` JSON API
- **新增**：`POST /api/v1/compare/stream` SSE 流式对比端点，HTML 页面支持边收边显示

## 要求

- Go **1.27.1**

## 快速开始

1. 复制配置（勿提交含真实 Key 的 `config.yaml`）：

```bash
cp config.example.yaml config.yaml
```

2. 设置 API Key（推荐环境变量）：

```bash
export OPENAI_API_KEY="your-key"
export ANTHROPIC_API_KEY="your-key"
export DEEPSEEK_API_KEY="your-key"
```

3. 构建并启动：

```bash
go build ./...
go run ./cmd/server
```

4. 浏览器打开 [http://localhost:8080/compare](http://localhost:8080/compare)

未设置 `CONFIG_PATH` 时，若不存在 `config.yaml` 会自动使用 `config.example.yaml`，进程可在无 Key 时启动；实际对比需在环境中配置 Key 或于本地 `config.yaml` 使用可选 `api_key` 字段。

## JSON API 示例

```bash
curl -s -X POST http://localhost:8080/api/v1/compare \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"用一句话解释 fan-out","model_ids":["openai-gpt","anthropic-claude"]}'
```

省略 `model_ids` 时使用配置中全部启用模型。

## SSE 流式对比

流式端点固定返回 `text/event-stream`，请求体与非流式 JSON 相同：

```bash
curl -N -X POST http://localhost:8080/api/v1/compare/stream \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"用一句话解释 fan-out","model_ids":["openai-gpt","deepseek-chat"]}'
```

事件说明（详见 `docs/SPEC.md`）：

| 事件 | 说明 |
|------|------|
| `model_start` | 某模型开始流式输出 |
| `delta` | 增量文本（含 `model_id` 与 `delta`） |
| `model_done` | 某模型完成（含完整结果、延迟、token） |
| `diff` | 行级 unified diff（至少两个模型成功后推送） |
| `done` | 整单完成 |
| `error` | 整单错误（如未选模型） |

HTML 对比页提供「流式对比」按钮，提交后会通过 `fetch` + ReadableStream 实时接收并展示各模型输出与最终 Diff；原有非流式表单保持不变。

## 文档

| 文档 | 说明 |
|------|------|
| [docs/PRD.md](docs/PRD.md) | 产品目标与验收 |
| [docs/DESIGN.md](docs/DESIGN.md) | 架构与模块 |
| [docs/SPEC.md](docs/SPEC.md) | 配置、API、用例（含 SSE 事件格式） |
| [AGENTS.md](AGENTS.md) | 项目规范 |

## 目录

```
cmd/server/          入口
internal/adapter/    厂商 HTTP 适配
internal/provider/   Provider 抽象
internal/fanout/     并发扇出
internal/compare/    Diff
internal/config/     YAML 配置
internal/handler/    HTTP
web/compare.html     对比页模板
```

## License

MIT — 见 [LICENSE](LICENSE)。
