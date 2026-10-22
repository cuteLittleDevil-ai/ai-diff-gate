# ai-diff-gate 技术规格（SPEC）

> 由原 `ai-diff-gate-PRD.md` Draft v0.1 拆分

| 字段 | 内容 |
|------|------|
| 版本 | v0.1 |
| 日期 | 2026-10-20 |
| 状态 | Draft |
| 对应 PRD | `docs/PRD.md` v0.2 |
| 对应 DESIGN | `docs/DESIGN.md` v0.1 |

本文描述可验收的技术契约。标注「建议默认，待确认」的条目实现前需与开放问题对齐拍板。

---

## 1. 范围与术语

### 1.1 范围

| 包含 | 不包含（见第 10 节） |
|------|----------------------|
| 配置驱动的多模型接入（至少 OpenAI、Anthropic、DeepSeek 协议形态） | 生产网关、用户体系、计费、HA |
| Fan-out、文本 Diff、HTML 展示 | 语义相似度、历史持久化 |
| JSON API、延迟/token 展示 | — |
| **SSE 流式对比**（`POST /api/v1/compare/stream`） | — |

### 1.2 术语

| 术语 | 含义 |
|------|------|
| Provider | 统一聊天调用抽象；一种协议/厂商对应一个实现 |
| Adapter | 将内部请求/响应映射到具体厂商 HTTP API 的实现层 |
| Fan-out | 同一 Prompt 并发调用多个 Provider 并汇总 |
| Diff | 对多份文本回复的差异计算与展示数据 |
| Compare 页 | 用户输入 Prompt、查看并排结果与 Diff 的 HTML 页面 |

---

## 2. 配置 schema（YAML 示例字段）

以下为**建议字段**，非最终锁定 schema。敏感信息处理对齐开放问题 Q5。

```yaml
# config.yaml 示例（建议结构，待确认）
server:
  addr: ":8080"          # 建议默认，待确认

# 全局超时等（也可放在 fanout 段）
fanout:
  timeout_seconds: 60    # 建议默认，待确认（Q1）
  # on_partial_failure: "continue"  # 建议默认 continue，待确认（Q1）
  # 可选值示意: continue | fail_all

models:
  - id: openai-gpt
    provider: openai
    display_name: "OpenAI"
    model: "gpt-4o-mini"           # 实际模型名以厂商为准
    base_url: "https://api.openai.com/v1"
    api_key_env: "OPENAI_API_KEY"  # 建议用环境变量名，待确认（Q5）
    # api_key: "sk-..."            # 明文不推荐；是否允许见 Q5

  - id: anthropic-claude
    provider: anthropic
    display_name: "Anthropic"
    model: "claude-3-5-sonnet-latest"
    base_url: "https://api.anthropic.com"
    api_key_env: "ANTHROPIC_API_KEY"

  - id: deepseek-chat
    provider: deepseek
    display_name: "DeepSeek"
    model: "deepseek-chat"
    base_url: "https://api.deepseek.com/v1"
    api_key_env: "DEEPSEEK_API_KEY"

compare:
  granularity: line      # 建议默认 line；可选 word | paragraph（Q2）
  ignore_whitespace: false  # 建议默认，待确认（Q2）
```

**建议默认，待确认**：

| 配置项 | 建议默认 | 对应问题 |
|--------|----------|----------|
| `server.addr` | `:8080` | — |
| `fanout.timeout_seconds` | `60` | Q1 |
| 部分失败策略 | 继续并标注失败（continue） | Q1 |
| `compare.granularity` | `line` | Q2 |
| `compare.ignore_whitespace` | `false` | Q2 |
| Key | 优先 `api_key_env`，避免明文入库 | Q5 |

---

## 3. 内部 ChatRequest / ChatResponse 字段建议

供 Provider / adapter 使用的内部结构（非对外 JSON 的最终契约；命名可按实现微调）。

### 3.1 ChatRequest（建议）

| 字段 | 类型（示意） | 说明 |
|------|--------------|------|
| `Model` | string | 目标模型名（来自配置） |
| `Messages` | []Message | 对话消息；MVP 至少支持单轮 user Prompt |
| `Temperature` | *float64 | 可选 |
| `MaxTokens` | *int | 可选 |

`Message` 建议至少包含：`Role`（user/assistant/system）、`Content`（string）。

### 3.2 ChatResponse（建议）

| 字段 | 类型（示意） | 说明 |
|------|--------------|------|
| `Content` | string | 归一化后的主要文本回复 |
| `Model` | string | 实际上游模型标识（若可得） |
| `LatencyMs` | int64 | 本次调用耗时（毫秒）；P1 展示 |
| `PromptTokens` | *int | 若上游提供 |
| `CompletionTokens` | *int | 若上游提供 |
| `TotalTokens` | *int | 若上游提供 |
| `RawError` | string | 失败时错误摘要（成功时可空） |

---

## 4. HTTP API

### 4.1 交付形态说明

| 形态 | MVP 要求 | 备注 |
|------|----------|------|
| HTML 表单 / 页面 | **建议必做** | 对齐 PRD F4 |
| JSON API | **建议默认同做，待确认（Q3）** | 便于调试与后续集成 |

若最终仅 HTML：第 4.3 节可降为「非交付」；验收用例以页面为准。

### 4.2 HTML（建议）

| 项 | 建议 |
|----|------|
| 方法 / 路径 | `GET /` 或 `GET /compare` 返回对比页；`POST /compare` 提交表单 |
| 表单字段（建议） | `prompt`（必填）；`models`（多选，模型 id 列表，对齐 Q4） |
| 成功响应 | HTML，含并排结果与 Diff（见第 7 节） |
| 部分失败 | 页面仍 200，失败模型区域展示错误信息（**建议默认，待确认** Q1） |

### 4.3 JSON API（若同时支持；建议路径与示例）

以下均为**建议**，待确认（Q3、Q6）。定位为对比专用 API，而非完整 OpenAI 兼容代理。

**请求**

- 方法 / 路径：`POST /api/v1/compare`
- `Content-Type: application/json`

```json
{
  "prompt": "用一句话解释什么是 fan-out。",
  "model_ids": ["openai-gpt", "anthropic-claude"]
}
```

若 `model_ids` 省略：行为待确认（Q4）——建议默认「使用配置中全部启用模型」。

**成功响应示例（建议）**

```json
{
  "prompt": "用一句话解释什么是 fan-out。",
  "results": [
    {
      "model_id": "openai-gpt",
      "display_name": "OpenAI",
      "ok": true,
      "content": "...",
      "latency_ms": 1234,
      "prompt_tokens": 20,
      "completion_tokens": 40,
      "total_tokens": 60
    },
    {
      "model_id": "anthropic-claude",
      "display_name": "Anthropic",
      "ok": false,
      "error": "timeout after 60s",
      "latency_ms": 60000
    }
  ],
  "diffs": [
    {
      "left_model_id": "openai-gpt",
      "right_model_id": "anthropic-claude",
      "granularity": "line",
      "unified_diff": "..."
    }
  ]
}
```

`diffs` 在双方均 `ok` 时生成；一方失败时可省略该对或标记不可比（**建议默认，待确认**）。

**错误响应（整单无法处理时，建议）**

- 400：缺少 `prompt`、未知 `model_ids` 等。
- 500：进程内未预期错误。
- 部分上游失败：**建议不**因此返回 5xx（对齐 Q1 的 continue）。

---

### 4.4 SSE 流式对比 API

新增端点：`POST /api/v1/compare/stream`

- 请求体与 `POST /api/v1/compare` 相同：`prompt` + 可选 `model_ids`。
- 响应头固定返回 `Content-Type: text/event-stream; charset=utf-8`。
- 多模型并发 fan-out，各模型 delta 交错推送；客户端断连时尽力取消未完成调用。

**事件格式（每行一个 SSE `data:` 负载，事件类型由 `event:` 指定）**

```text
event: model_start
data: {"model_id":"openai-gpt","display_name":"OpenAI GPT-4o mini"}

event: delta
data: {"model_id":"openai-gpt","delta":"Fan-out"}

event: model_done
data: {"model_id":"openai-gpt","display_name":"OpenAI GPT-4o mini","ok":true,"content":"...","latency_ms":123,"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}

event: diff
data: {"left_model_id":"openai-gpt","right_model_id":"deepseek-chat","granularity":"line","unified_diff":"..."}

event: done
data: {"ok":true}
```

**事件说明**

| 事件 | 触发时机 | data 字段 |
|------|----------|-----------|
| `model_start` | 某模型开始流式调用 | `model_id`, `display_name` |
| `delta` | 收到该模型增量文本 | `model_id`, `delta` |
| `model_done` | 某模型调用完成（成功或失败） | 与 JSON API 单条 `results` 字段一致，额外含 `content`/`error` |
| `diff` | 全部模型结束后，对成功结果两两生成行级 Diff | `left_model_id`, `right_model_id`, `granularity`, `unified_diff` |
| `done` | 全部事件已推送 | `ok` |
| `error` | 整单无法处理（如无模型） | `message` |

**curl 示例**

```bash
curl -N -X POST http://localhost:8080/api/v1/compare/stream \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"用一句话解释 fan-out","model_ids":["openai-gpt","deepseek-chat"]}'
```

---

## 5. Fan-out 行为

| 行为 | 建议默认（待确认） |
|------|-------------------|
| 并发 | 对所选模型并发调用；每个调用独立 context |
| 超时 | 单模型 / 整单超时建议默认 **60s**（Q1） |
| 部分失败 | **继续**：成功结果照常进入 Diff/展示，失败项带错误信息（Q1） |
| 取消 | 客户端断开时取消未完成调用（实现尽力而为） |
| 模型集合 | 来自请求勾选或配置；至少 2 个启用模型方可有意义对比 |

---

## 6. Diff 行为

| 行为 | 建议默认（待确认） |
|------|-------------------|
| 粒度 | **行级（line）**（Q2）；可选 word / paragraph |
| 空白 | **不忽略**空白差异（`ignore_whitespace: false`）（Q2） |
| 对比对象 | 仅对 `ok=true` 的文本 `content` 两两或相对基准模型对比 |
| 输出 | 至少提供可用于 HTML 高亮或统一 diff 文本的结构 |

语义相似度不在本 SPEC 范围内。

---

## 7. HTML 页面最小展示要素

Compare 页 MVP 至少包含：

1. Prompt 输入区（多行文本）与提交控件。
2. 模型选择：多选列表或等价控件（若 Q4 定为配置写死，可改为只读展示本轮模型集）。
3. 各模型结果区：展示名、状态（成功/失败）、正文；失败时展示错误摘要。
4. Diff 区：至少一对成功结果的文本差异可视化或统一 diff。
5. （P1）延迟、token：有则展示，无则隐藏或显示「N/A」。

不要求精美 UI；单 HTML 文件即可。

---

## 8. 可观测字段

| 字段 | 来源 | 优先级 | 展示 |
|------|------|--------|------|
| `latency_ms` | 客户端侧对单次 Provider 调用计时 | P1（PRD F6） | HTML / JSON |
| `prompt_tokens` / `completion_tokens` / `total_tokens` | 上游用量字段（若有） | P1 | 有则展示 |
| 错误摘要 | adapter / 超时 / HTTP 错误 | P0 | 失败模型必显 |

结构化日志格式本阶段不强制；本地 Demo 可用标准日志输出关键请求与各模型结果状态。

---

## 9. 验收用例表

| 编号 | Given | When | Then |
|------|-------|------|------|
| AC1 | 配置中至少有 2 个不同 `provider` 的模型，且 Key 可用 | 提交同一 Prompt 对比二者 | 两个成功结果均返回；页面或 JSON 可见两份正文 |
| AC2 | 同上 | 一次对比请求 | 总耗时体现并发（明显小于两模型串行之和，网络抖动除外） |
| AC3 | 至少两模型成功 | 打开/返回对比页 | 可见并排结果与基本文本 Diff |
| AC4 | 仓库可构建 | 按文档执行启动命令（如 `go run ./cmd/server`） | 进程监听配置端口，页面可打开 |
| AC5 | 新增第三家仅需新 adapter + 配置项（设计意图） | 按 DESIGN 扩展路径评估 | 无需改写 fanout/compare 核心逻辑即可接入（代码评审级） |
| AC6 | 一模型故意错误 Key 或不可达 | 发起对比 | **建议**：另一模型仍展示成功；失败模型有错误信息（对齐 Q1，待确认） |
| AC7 | 超时配置为较短值，一模型极慢 | 发起对比 | 慢模型超时失败，行为符合 Q1 所定策略 |
| AC8 | （若启用 JSON）合法 `POST /api/v1/compare` | 提交 JSON | 返回第 4.3 节形态的结果；字段含 `results` |
| AC9 | （P1）上游返回 usage | 成功调用 | 页面或 JSON 可见 token 相关字段 |
| AC10 | 缺少 Prompt | 提交 | 400 或页面内校验提示，不发起无意义上游调用 |

产品级 5 条验收与 PRD 第 7 节一致；上表为可执行细化。

---

## 10. Out of scope（技术边界）

本 SPEC 明确不覆盖：

- 生产级鉴权、多租户、额度与计费。
- 集群部署、高可用、自动扩缩。
- 完整 OpenAI 兼容透明代理（除非 Q6 拍板并另开规格）。
- 语义相似度、向量检索、请求历史库。
- 密钥托管、密钥轮转平台。
- 非 Go 实现、移动端原生客户端。

---

## 11. 开放问题（与 PRD / DESIGN 对齐编号）

| 编号 | 问题 | SPEC 建议默认 |
|------|------|---------------|
| Q1 | 部分模型失败语义与超时 | continue + 标注失败；超时 60s |
| Q2 | Diff 粒度与空白处理 | 行级；不忽略空白 |
| Q3 | 仅 HTML vs 同时 JSON API | HTML 必做；JSON 建议同做（`POST /api/v1/compare`） |
| Q4 | 模型选择：页面勾选 vs 配置写死 | 配置声明可用集 + 页面勾选子集；省略勾选则用全部启用模型 |
| Q5 | Key：明文 YAML / 环境变量 / `.env` | 优先 `api_key_env`；允许本地明文仅作个人 Demo 且勿提交仓库 |
| Q6 | OpenAI 兼容透明代理 vs 对比专用 API | MVP 对比专用；不做完整兼容代理 |
| Q7 | 产品名是否还会改 | 文档与页面标题暂用 `ai-diff-gate`；改名不改契约字段 |

---

## 12. 文档关系

| 文档 | 用途 |
|------|------|
| `docs/PRD.md` | 产品目标、非目标、功能优先级、产品验收 |
| `docs/DESIGN.md` | 架构、模块、选型建议与取舍 |
| `docs/SPEC.md`（本文） | 配置、API、行为默认、用例、技术边界 |
| `ai-diff-gate-PRD.md` | 原 Draft v0.1 混杂稿（保留） |
