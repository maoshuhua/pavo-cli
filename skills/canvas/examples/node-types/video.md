# `video`：视频生成节点

**用户请求**： “基于画布中的关键帧创建一个 8 秒推进镜头并生成。”

**覆盖**：图片引用、实时视频约束、动作时间线、单视频生成下载。

**前置条件**：已有关键帧 image/upload 节点；用户明确要求生成视频。

```bash
pavo canvas model list --scene canvas_video

pavo canvas node create \
  --type video \
  --name "车站推进镜头" \
  --prompt "【时长】8 秒；【起始画面】沿用关键帧人物、产品和车站构图；【动作时间线】0–3 秒人物停步，3–6 秒抬眼，6–8 秒轻微呼吸并稳定收束；【运镜】全程缓慢推近，无突然变焦；【光线】冷蓝雨幕与暖黄侧光连续；【声音】雨声和远处列车声；【结尾】停在近景；【负面约束】闪烁、跳切、身份漂移、服装漂移、背景漂移、镜头抖动" \
  --model "LIVE_VIDEO_MODEL_CODE" \
  --data '{"params":{"duration":8,"settings":{"ratio":"16:9","resolution":"hd","generateAudio":false}}}'

pavo canvas edge add --source "KEYFRAME_NODE_KEY" --target "VIDEO_NODE_KEY"
pavo canvas validate "VIDEO_NODE_KEY" --strict
pavo canvas run "VIDEO_NODE_KEY" --download
```

**输出与验收**：模型实时约束接受时长/比例/分辨率；真实 connection 存在；任务成功后视频结果同时有远程 `url` 和绝对 `local_path`。

**失败处理**：首尾帧或多模态 guide 场景优先使用 shortcut，不要只靠普通 edge 猜输入语义。模型不支持 8 秒时接受实时约束给出的合法时长，不能强塞无效参数。
