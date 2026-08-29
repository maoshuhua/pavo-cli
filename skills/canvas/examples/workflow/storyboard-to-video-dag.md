# 工作流：Storyboard → 关键帧/视频节点 → DAG 生成

**用户请求**： “把已经确认的 6 镜分镜搭成关键帧和视频，并按依赖全部生成下载。”

**覆盖**：结构化分镜同步、图片→视频依赖、group 范围 DAG、拓扑执行、并行下载。

**前置条件**：storyboard 节点已经 `validate --strict` 通过；用户明确要求生成图片和视频。

```bash
# 1) 查询两类实时模型
pavo canvas model list --scene canvas_image
pavo canvas model list --scene canvas_video
pavo canvas model show "LIVE_IMAGE_MODEL_CODE" --scene canvas_image
pavo canvas model show "LIVE_VIDEO_MODEL_CODE" --scene canvas_video

# 2) dry-run 后同步每镜 image/video 节点；记录 stdout.group_key
pavo canvas storyboard build "STORYBOARD_NODE_KEY" \
  --image-model "LIVE_IMAGE_MODEL_CODE" \
  --with-video \
  --video-model "LIVE_VIDEO_MODEL_CODE" \
  --dry-run \
  --strict
pavo canvas storyboard build "STORYBOARD_NODE_KEY" \
  --image-model "LIVE_IMAGE_MODEL_CODE" \
  --with-video \
  --video-model "LIVE_VIDEO_MODEL_CODE" \
  --strict

# 3) 生成前校验并按 group 做 DAG 计划；记录 stdout.plan_id
pavo canvas validate --all --strict
pavo canvas dag plan --group "STORYBOARD_GROUP_KEY"

# 4) 用户已明确要求生成，执行并下载
pavo canvas dag run \
  --plan "PLAN_ID" \
  --max-parallel 4 \
  --download \
  --output-dir "ABSOLUTE_OUTPUT_DIRECTORY"
```

**输出与验收**：两个模型 `available=true` 且时长/输入模式符合 `guidance`；build `lint.quality_ready=true`；计划的每个视频节点依赖同 shot 图片节点；`levels` 先图片后视频，同层可并行；最终 `run.nodes[]` 没有 `failed`/意外 `skipped`；成功节点资源有绝对 `local_path`；返回 `canvas_url`。

**失败处理**：上游图片失败导致视频 `skipped` 时不要单独强跑视频；先修复失败图片，再按失败恢复规则重新规划或恢复。执行前图发生变化并返回 `replan_required` 时使用对应失败案例。
