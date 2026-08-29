# 工作流：应用角色设定 Shortcut

**用户请求**： “基于这张人物参考图生成角色设定图，后面镜头都要保持同一个人。”

**覆盖**：实时 shortcut 发现、skill prompt segment、引用边、原子 dry-run、可选生成。

**前置条件**：画布已存在人物参考 upload/image 节点；用户已明确要求生成角色设定图。

```bash
# 1) 实时发现当前 image skill；从输出选择角色设定 code
pavo canvas shortcut list --kind skill --type image
pavo canvas shortcut show "CHARACTER_SETTING_CODE"

# 2) 选择实时图片模型
pavo canvas model list --scene canvas_image

# 3) 先验证原子 batch，不改画布
pavo canvas shortcut apply "CHARACTER_SETTING_CODE" \
  --source "CHARACTER_REFERENCE_NODE" \
  --name "林岚 · 角色设定" \
  --prompt "固定身份：黑色短发、棕色眼睛、左眼下方小痣；固定米色风衣、深棕皮靴与银色腕表；正面自然站姿，干净中性背景；避免五官、发型、服装纹样和年龄漂移" \
  --model "LIVE_IMAGE_MODEL_CODE" \
  --dry-run

# 4) 实际原子搭图并立即执行返回的目标节点
pavo canvas shortcut apply "CHARACTER_SETTING_CODE" \
  --source "CHARACTER_REFERENCE_NODE" \
  --name "林岚 · 角色设定" \
  --prompt "固定身份：黑色短发、棕色眼睛、左眼下方小痣；固定米色风衣、深棕皮靴与银色腕表；正面自然站姿，干净中性背景；避免五官、发型、服装纹样和年龄漂移" \
  --model "LIVE_IMAGE_MODEL_CODE" \
  --run --download
```

**输出与验收**：`shortcut.code` 与实时 code 一致；`aliases` 有新节点；`run_node_key` 非空；生成分支的 `run.task.failed=false` 且结果有 `local_path`。目标节点的 prompt segments 同时包含 skill code 和完整 text prompt。

**失败处理**：code 不存在时重新 `shortcut list`；不要退回手写旧 skill code。如果另一请求只要求搭建、不要求生成，使用同一命令但省略 `--run --download`。
