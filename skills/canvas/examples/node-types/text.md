# `text`：文本生成节点

**用户请求**： “在画布上创建一个广告旁白文本节点，并生成文案。”

**覆盖**：实时文本模型、组合式 prompt、单节点校验与生成。

**前置条件**：画布已绑定；用户明确要求生成文本。

```bash
# 1) 从实时 textModes/textModels 选择在线 modelCode
pavo canvas tool-specs

# 2) 创建节点并记录 node_key
pavo canvas node create \
  --type text \
  --name "雨夜广告旁白" \
  --prompt "为 15 秒雨夜城市产品广告写一句 30 字以内的克制旁白；语气冷静但有温度；不要堆砌形容词，不要解释创作过程" \
  --model "LIVE_TEXT_MODEL_CODE"

pavo canvas validate "TEXT_NODE_KEY" --strict
pavo canvas run "TEXT_NODE_KEY"
```

**输出与验收**：任务成功后节点 `data.content` 是生成文本，`mode=authoring`，网页刷新仍可见；`task.failed=false`；返回 `canvas_url`。

**失败处理**：文本模型不存在或离线时重新读 `tool-specs`。只想录入静态文字而非生成时，不应执行 `run`。
