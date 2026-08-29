# 失败恢复：Canvas Schema 校验未通过

**用户请求**： “运行前帮我检查画布；有问题就定位并修好。”

**覆盖**：结构化 validation issues、节点精确读取、prompt/model/skill 修复、复验。

**前置条件**：用户已授权修复当前画布配置，但未授权生成或删除节点。

```bash
# 1) 输出完整问题；--strict 在存在 error 时返回非零状态
pavo canvas validate --all --strict

# 2) 对每个 valid=false 的节点精确读取，保留未知字段
pavo canvas node get "INVALID_NODE_KEY"

# 3A) 修复缺失/失效模型：先查实时列表，再只更新 model
pavo canvas model list --scene canvas_image
pavo canvas node update "INVALID_NODE_KEY" --model "LIVE_IMAGE_MODEL_CODE"

# 3B) 修复 text prompt，同时保留已有 skill/media segments
pavo canvas node update "INVALID_NODE_KEY" \
  --prompt "完整主体、场景、构图、动作、光线、风格和负面约束"

# 3C) skill code 失效时先发现实时 code，再明确替换完整 segments，移除旧 code
pavo canvas shortcut list --kind skill --type image
pavo canvas node update "INVALID_NODE_KEY" \
  --prompt-segments '[{"type":"skill","code":"LIVE_SKILL_CODE"},{"type":"text","content":"完整主体、场景、构图、动作、光线、风格和负面约束"}]'

# 4) 复验
pavo canvas validate --all --strict
```

**输出与验收**：根据 `nodes[].issues[].path/message` 修复对应字段；最终 `valid=true`，没有 graph error；全过程没有执行 run/delete。

**失败处理**：warning 不会令 `valid=false`，应按场景判断；不要为消除 warning 随意覆盖服务端默认值。若 graph error 是环或悬空边，先读取 edge list 并报告具体结构，删除边需要用户明确授权。
