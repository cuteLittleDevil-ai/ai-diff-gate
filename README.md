# ai-diff-gate

轻量 Go Demo：配置驱动接入多模型，对同一 Prompt 并发扇出，行级文本 Diff，HTML 并排展示与 JSON 对比 API。

## 功能

- OpenAI、Anthropic、DeepSeek、MiniMax 协议适配（可扩展 adapter）
- Fan-out 并发调用，单模型超时默认 60s，部分失败 continue
- 行级 unified diff（不忽略空白）
- `GET` / `POST /compare` 对比页，`POST /api/v1/compare` JSON API

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
export MINIMAX_API_KEY="your-key"
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
  -d '{"prompt":"用一句话解释 fan-out","model_ids":["openai-gpt","anthropic-claude","minimax-m3.1-flash"]}'
```

省略 `model_ids` 时使用配置中全部启用模型。

## 文档

| 文档 | 说明 |
|------|------|
| [docs/PRD.md](docs/PRD.md) | 产品目标与验收 |
| [docs/DESIGN.md](docs/DESIGN.md) | 架构与模块 |
| [docs/SPEC.md](docs/SPEC.md) | 配置、API、用例 |
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
