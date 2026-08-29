# 工作流：用 NDJSON 原子搭建节点、连线和 Group

**用户请求**： “一次性在画布里搭一个文案→主视觉→视频的工作流，先预览，不生成。”

**覆盖**：文件式 NDJSON、别名、模型实时校验、原子 dry-run、图结构验收。

**前置条件**：画布已绑定；已从实时模型列表选择图片和视频 model code。用户只要求搭图，不执行生成。

```bash
pavo canvas tool-specs
pavo canvas model list --scene canvas_image
pavo canvas model list --scene canvas_video
```

将以下内容保存为 `workflow.ndjson`，每行只能有一个 JSON object：

```ndjson
{"op":"node.create","as":"copy","type":"text","name":"广告文案","prompt":"为雨夜城市产品广告写一句克制旁白","model":"LIVE_TEXT_MODEL_CODE"}
{"op":"node.create","as":"kv","type":"image","name":"雨夜主视觉","prompt":"产品结构保持一致；雨夜城市橱窗，中近景，冷蓝与品红轮廓光，写实商业摄影，16:9；避免文字、标识和结构漂移","model":"LIVE_IMAGE_MODEL_CODE"}
{"op":"edge.add","source":"$copy","target":"$kv","role":"prompt","media_order":0}
{"op":"node.create","as":"video","type":"video","name":"主视觉视频","prompt":"8 秒；从主视觉起幅；0–3 秒缓慢推进，3–6 秒雨滴掠过镜头，6–8 秒稳定落版；避免闪烁、跳切和产品漂移","model":"LIVE_VIDEO_MODEL_CODE"}
{"op":"edge.add","source":"$kv","target":"$video","role":"reference","media_order":0}
{"op":"group.create","as":"campaign","members":["$copy","$kv","$video"],"name":"雨夜广告工作流","mode_code":"campaign"}
```

```bash
# 1) 只编译和校验，不改画布
pavo canvas apply --file workflow.ndjson --dry-run

# 2) 用户确认要求就是搭建该图时，原子提交；仍不生成
pavo canvas apply --file workflow.ndjson

# 3) 验证最终图
pavo canvas validate --all --strict
pavo canvas node list
pavo canvas edge list
```

临时管道也可使用 `Get-Content -Raw workflow.ndjson | pavo canvas apply --stdin --dry-run`；可审阅和复用的工作流优先 `--file`。`--file` 与 `--stdin` 不能同时传。

**输出与验收**：dry-run `counts` 为 3 个 node.create、2 个 edge.add、1 个 group.create；实际输出 `aliases` 映射到稳定 node key；一次 batch 全部成功；未调用任何 run 命令。

**失败处理**：任一行报错时按返回的行号和 op 修复文件，不能把失败行删掉后提交残缺图。NDJSON 不承载上传、生成或 artifact 删除。
