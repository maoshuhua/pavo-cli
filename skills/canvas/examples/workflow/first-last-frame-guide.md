# 工作流：首尾帧 Guide → 视频节点

**用户请求**： “用画布里的首帧和尾帧做一个 8 秒过渡视频。”

**覆盖**：实时 guide 输入发现、多输入绑定、视频参数、模板 group、可选立即生成。

**前置条件**：画布已有首帧和尾帧节点；两者应是不同的 image/upload 节点。

```bash
# 1) 实时发现 guide，并读取 required_inputs[].key
pavo canvas shortcut list --kind guide --type video
pavo canvas shortcut show "FIRST_LAST_FRAME_GUIDE_CODE"

# 2) 查询实时视频模型
pavo canvas model list --scene canvas_video

# 3) 用 show 返回的实际 key 绑定输入；先 dry-run
pavo canvas shortcut apply "FIRST_LAST_FRAME_GUIDE_CODE" \
  --input "FIRST_INPUT_KEY=FIRST_FRAME_NODE" \
  --input "LAST_INPUT_KEY=LAST_FRAME_NODE" \
  --name "雨夜变装过渡" \
  --prompt "8 秒连续镜头：0–2 秒保持首帧人物身份和服装；2–6 秒镜头缓慢推进，雨滴与灯光逐步改变；6–8 秒自然收束到尾帧构图；避免跳切、闪烁、身份漂移、背景结构突变" \
  --model "LIVE_VIDEO_MODEL_CODE" \
  --dry-run

# 4) 用户已明确要求视频，实际搭图、执行并下载
pavo canvas shortcut apply "FIRST_LAST_FRAME_GUIDE_CODE" \
  --input "FIRST_INPUT_KEY=FIRST_FRAME_NODE" \
  --input "LAST_INPUT_KEY=LAST_FRAME_NODE" \
  --name "雨夜变装过渡" \
  --prompt "8 秒连续镜头：0–2 秒保持首帧人物身份和服装；2–6 秒镜头缓慢推进，雨滴与灯光逐步改变；6–8 秒自然收束到尾帧构图；避免跳切、闪烁、身份漂移、背景结构突变" \
  --model "LIVE_VIDEO_MODEL_CODE" \
  --run --download
```

**输出与验收**：dry-run 的 `request` 包含模板节点、两条输入边及 group；实际结果有 `run_node_key` 和 `canvas_url`；最终任务成功且视频结果有 `local_path`。

**失败处理**：不要把案例中的 first/last 当固定 key，始终以本次 `shortcut show` 为准。缺输入时补 `--input KEY=NODE`，不要使用示例素材替代用户素材；只有用户明确接受模板示例时才加 `--use-example-input`。
