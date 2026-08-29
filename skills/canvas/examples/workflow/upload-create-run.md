# 工作流：上传参考图 → 创建图片节点 → 连线 → 生成并下载

**用户请求**： “把这张本地产品图放进画布，按它生成一张雨夜霓虹主视觉，并下载结果。”

**覆盖**：本地上传、实时模型发现、节点创建、引用连线、Schema 校验、单节点生成和本地结果。

**前置条件**：画布已绑定；用户已提供本地图片绝对路径，并明确要求生成。

```bash
# 1) 上传并记录 stdout.node_key
pavo canvas upload --file "ABSOLUTE_PATH_TO_PRODUCT_IMAGE" --name "产品参考图"

# 2) 查询实时图片模型，选择 allowed 且在线的 MODEL_CODE
pavo canvas model list --scene canvas_image

# 3) 创建目标图片节点并记录 stdout.node_key
pavo canvas node create \
  --type image \
  --name "雨夜霓虹主视觉" \
  --prompt "产品外观与标识保持一致；雨夜城市橱窗，中近景，主体位于右侧三分线；冷蓝环境光与品红轮廓光；写实商业摄影，16:9；避免文字变形、标识漂移和产品结构变化" \
  --model "LIVE_IMAGE_MODEL_CODE"

# 4) 使用前两步返回的 node_key 建立真实引用边
pavo canvas edge add --source "UPLOAD_NODE_KEY" --target "IMAGE_NODE_KEY"

# 5) 生成前校验；用户已要求生成，所以执行并下载
pavo canvas validate "IMAGE_NODE_KEY" --strict
pavo canvas run "IMAGE_NODE_KEY" --download
```

**输出与验收**：

- `edge add` 返回 `connection_id`；重新 `node get IMAGE_NODE_KEY` 时可看到 source 关系。
- `run.task.failed` 为 `false`；成功结果有远程 `url` 和绝对 `local_path`。
- `canvas_url` 可打开对应网页画布，页面能看到上传节点、生成节点和连线。

**失败处理**：模型校验失败时重新查询 `canvas_image`，不要改用其他生成命令的模型 code。任务响应不明确时按返回的 `task_id` 查询，不加 `--force` 盲目重跑。
