# `image`：图片生成节点

**用户请求**： “在画布建一张电影感产品关键帧，先搭节点。”

**覆盖**：实时图片模型、八段式单镜 prompt、生成与搭图边界。

**前置条件**：画布已绑定；本案例用户只要求搭节点。

```bash
pavo canvas model list --scene canvas_image

pavo canvas node create \
  --type image \
  --name "车站产品关键帧" \
  --prompt "【分镜目标】建立雨夜车站与产品；【主体一致性】银色耳机结构、材质和标识固定；【场景一致性】潮湿老车站、雨夜、老式顶棚；【构图与机位】中近景、平视、主体位于右侧三分线；【动作与表情】雨滴落在外壳，产品静止；【光线与色彩】冷蓝环境光、暖黄轮廓光；【统一视觉风格】写实电影广告、16:9、高细节；【负面约束】文字、水印、标识漂移、结构变形、背景漂移" \
  --model "LIVE_IMAGE_MODEL_CODE"

pavo canvas validate "IMAGE_NODE_KEY" --strict
```

**输出与验收**：节点类型为 image，`params.model` 是实时可用模型，prompt 是 text segment，校验有效；没有创建生成任务。

**失败处理**：需要角色/产品/场景 preset 时不要继续手写 skill segment，改用 `shortcut list/show/apply` 对应案例。
