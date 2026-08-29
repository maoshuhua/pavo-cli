# CLI-only `storyboard`：结构化分镜

**用户请求**： “我已经有一份结构化分镜 JSON，把它导入画布并搭成关键帧节点。”

**覆盖**：JSON Schema、导入、持久化 text 节点、幂等 shot build、导出复核。

**前置条件**：本地 JSON 满足 `pavo.storyboard/v1`；用户只要求导入和搭图。

```bash
# 1) 查看权威 Schema，并在接触画布前离线检查/编译
pavo canvas storyboard schema
pavo canvas storyboard lint "ABSOLUTE_STORYBOARD_JSON_PATH" --strict
pavo canvas storyboard compile "ABSOLUTE_STORYBOARD_JSON_PATH" --kind image --strict

# 2) 导入并记录 node_key
pavo canvas storyboard import "ABSOLUTE_STORYBOARD_JSON_PATH" \
  --name "雨夜重逢 · Storyboard"

pavo canvas storyboard show "STORYBOARD_NODE_KEY"
pavo canvas storyboard validate "STORYBOARD_NODE_KEY" --strict

# 3) 选择实时图片模型，dry-run 后搭建；不生成
pavo canvas model list --scene canvas_image
pavo canvas model show "LIVE_IMAGE_MODEL_CODE" --scene canvas_image
pavo canvas storyboard build "STORYBOARD_NODE_KEY" \
  --image-model "LIVE_IMAGE_MODEL_CODE" \
  --dry-run \
  --strict
pavo canvas storyboard build "STORYBOARD_NODE_KEY" \
  --image-model "LIVE_IMAGE_MODEL_CODE" \
  --strict

# 4) 可选导出复核
pavo canvas storyboard export "STORYBOARD_NODE_KEY" \
  --output "ABSOLUTE_EXPORT_JSON_PATH"
```

**输出与验收**：离线 lint `quality_ready=true`，compile 每镜有非空 `image_prompt`；模型 `available=true`；导入节点 `data.pavo_storyboard.schema_version=pavo.storyboard/v1`；build 每个 shot 对应一个稳定 `image_node_key`，返回 group；再次 build 为 `changed:false`；没有执行生成。

**失败处理**：lint/import 错误必须修复 JSON 中报告的 path；quality warning 修改结构化字段或补真实参考 node key，不要把不合规自然语言直接塞进 `data.pavo_storyboard`。再次 build 会同步同一 shot 的节点，不应复制一套新节点。
