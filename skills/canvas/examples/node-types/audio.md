# `audio`：音频生成节点

**用户请求**： “在画布上生成一段雨夜车站氛围音乐。”

**覆盖**：实时音频模型、具体声音提示词、单节点生成下载。

**前置条件**：画布已绑定；用户明确要求生成音频。

```bash
pavo canvas model list --scene canvas_audio

pavo canvas node create \
  --type audio \
  --name "雨夜车站氛围乐" \
  --prompt "低速极简氛围音乐；稀疏钢琴、低频合成器铺底、远处列车金属摩擦质感；克制、潮湿、略带怀旧；不要人声，不要强鼓点，不要突然高潮" \
  --model "LIVE_AUDIO_MODEL_CODE"

pavo canvas validate "AUDIO_NODE_KEY" --strict
pavo canvas run "AUDIO_NODE_KEY" --download
```

**输出与验收**：任务成功，结果包含音频 URL 和本地 `local_path`；画布节点执行状态与结果已回写。

**失败处理**：模型列表没有可用项时停止并报告账号/在线约束，不从图片或视频模型列表借用 code。
