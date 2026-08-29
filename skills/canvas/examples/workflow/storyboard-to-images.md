# 工作流：剧情 → 结构化 Storyboard → 关键帧节点

**用户请求**： “把雨夜重逢的剧情拆成 6 个连续镜头，先把关键帧工作流搭好，不要生成图片。”

**覆盖**：严格 storyboard 请求、文本生成、Schema finalize、提示词编译、图片节点/group 搭建。

**前置条件**：画布已绑定；用户已授权生成分镜脚本文本，但明确不生成图片。

```bash
# 1) 创建结构化分镜请求节点，记录 stdout.node_key
pavo canvas storyboard create \
  --profile cinematic \
  --title "雨夜重逢" \
  --brief "两位多年未见的旧友在老车站重逢；写实电影感；林岚始终是黑色短发、米色风衣；车站空间、雨势、冷蓝与暖黄光色连续；剧情从迟疑到释然" \
  --shots 6

# 2) 生成文本、等待并自动 finalize；输出必须通过 pavo.storyboard/v1
pavo canvas storyboard generate "STORYBOARD_NODE_KEY"
pavo canvas storyboard show "STORYBOARD_NODE_KEY"
pavo canvas storyboard validate "STORYBOARD_NODE_KEY" --strict

# 3) 导出并离线检查最终图片提示词，禁止 Agent 另行自由润色
pavo canvas storyboard export "STORYBOARD_NODE_KEY" --output storyboard.json
pavo canvas storyboard lint storyboard.json --strict
pavo canvas storyboard compile storyboard.json --kind image --strict --output image-prompts.json

# 4) 查询实时图片模型并解释实际默认参数
pavo canvas model list --scene canvas_image
pavo canvas model show "LIVE_IMAGE_MODEL_CODE" --scene canvas_image

# 5) 先查看 build batch，再实际搭建关键帧；不运行这些图片节点
pavo canvas storyboard build "STORYBOARD_NODE_KEY" \
  --image-model "LIVE_IMAGE_MODEL_CODE" \
  --dry-run \
  --strict
pavo canvas storyboard build "STORYBOARD_NODE_KEY" \
  --image-model "LIVE_IMAGE_MODEL_CODE" \
  --strict

# 6) 检查整张图
pavo canvas validate --all --strict
```

**输出与验收**：storyboard 有 6 条唯一且连续的 shot ID/order；人物使用 `character_ids`，产品/道具使用 `subject_ids`；`lint.quality_ready=true`；compile 返回 6 个非空 `image_prompt`，每条都包含角色/固定主体、场景、构图、光线、统一风格和负面约束；model `available=true`；build 返回 6 个 `assets[].image_node_key`、非空 `group_key` 和 `changed:true`。相同输入再次 build 返回 `changed:false`；图校验有效；没有调用图片节点 `run` 或 `dag run`。

**失败处理**：`generate`/`finalize` 报 Schema 错误时停下并报告具体字段，不从自然语言猜测补齐。跨镜角色/场景 reference advisory 在效果要求高时优先通过 upload/shortcut 资产和真实 `reference_node_keys` 解决；warning 必须修改对应结构化字段。编辑 JSON 后用 `storyboard import --node` 回写，再重新 lint/compile/build。
