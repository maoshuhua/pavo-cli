---
name: short-drama
description: 使用 PAVO CLI 创建、续写、修改和恢复多轮短剧创作会话。适用于用户要求创作短剧、写短剧剧本、继续某个短剧、回答短剧创作问题、调整短剧风格或获取短剧生成结果时。短剧必须通过 pavo short-drama 命令完成，不使用通用 design stream。
---

# PAVO 短剧

使用 `pavo short-drama` 处理短剧创作。短剧是服务端维护状态的多轮会话：首次创建会话，后续补充、选择、确认与修改都复用同一个 `conversation_id`。不要改用通用 `pavo stream`，不要调用其他图像或视频服务，也不要自行代写、扩写或重组用户的短剧需求。

## 命令

1. `pavo short-drama start`
2. `pavo short-drama reply`
3. `pavo short-drama resume`
4. `pavo short-drama status`
5. `pavo short-drama result`
6. `pavo download-result`

## 首次创作

原样传递用户的短剧需求。需要本地参考文件时，为每个文件增加一个 `--file`。命令会创建会话，并以 `mode: "short_drama"`、`extra_context.agent_params.image_model_code: "agnes-image"` 和 `video_model_code: "agnes-video"` 发起首轮请求：

```bash
pavo short-drama start --prompt "USER_PROMPT"
```

默认从 stdout 的最终 JSON 记录并向用户返回 `conversation_id`。优先读取 `assistant_text`（完整服务端文本）、`assistant_messages` 和 `review`，将服务端给出的剧本、角色、场景或确认详情完整展示给用户；不要只做一句概括。过程事件会写入 stderr，仅用于诊断。

## 模型选择

默认使用 `agnes-image` 与 `agnes-video`。用户未要求选择或切换模型时，不要主动展示模型表，也不要自行替换默认值。

用户明确要求选择、比较或切换图像/视频模型时，展示下表并让用户分别选择一项图像模型和一项视频模型；不要代替用户决定。仅使用表中的 code，不要编造其他 code。服务端若返回模型不可用、无权限或不支持的错误，如实报告并等待用户重新选择。

图像模型：

| Code | 名称 |
| --- | --- |
| `seedream5-0-pro` | Seedream 5.0 Pro |
| `nano2-lite` | Nano Banana 2 Lite（Gemini 3.1 Flash Lite Image） |
| `seedream4-5` | Seedream 4.5 |
| `seedream4-0` | Seedream 4.0 |
| `gpt-image-2` | GPT Image 2 |
| `nanopro` | Nano Banana Pro（Gemini 3 Pro Image） |
| `nano2` | Nano Banana 2（Gemini 3.1 Flash Image） |
| `seedream5-0` | Seedream 5.0 Lite |
| `agnes-image` | Agnes Image（通常对应 Agnes Image 2.1 Flash） |

视频模型：

| Code | 名称 |
| --- | --- |
| `seedance-2-0-mini` | Seedance 2.0 Mini |
| `happyhorse-1-1` | HappyHorse 1.1 |
| `wan-2-7` | Wan 2.7 |
| `happyhorse-1-0` | HappyHorse 1.0 |
| `seedance-2-0-fast` | Seedance 2.0 Fast |
| `seedance-2-0` | Seedance 2.0 |
| `agnes-video` | Agnes Video V2.0 |
| `seedance-1-5-pro` | Seedance 1.5 Pro |

只有 `start` 和 `reply` 可设置模型；`resume` 仅恢复已有任务，不能改模型。选择或切换时，始终显式传递完整的一对 `--image-model-code` 与 `--video-model-code`：

```bash
pavo short-drama reply \
  --conversation-id "CONVERSATION_ID" \
  --prompt "USER_REPLY" \
  --image-model-code "seedream5-0-pro" \
  --video-model-code "seedance-2-0"
```

## 后续轮次

服务端要求用户补充信息、选择创作方向或确认下一步时，展示当前问题与选项并等待用户回答。不要替用户选择风格、剧情、角色或镜头。收到回答后，使用同一会话 ID：

```bash
pavo short-drama reply \
  --conversation-id "CONVERSATION_ID" \
  --prompt "USER_REPLY"
```

每次 `reply` 仍会携带短剧模式和默认的 Agnes 图片、视频模型。不要为后续轮次创建新会话，也不要用 `pavo conversation create` 代替 `reply`。

## 中断、状态和结果

流断开或已有短剧轮次仍在运行时，保留同一 `conversation_id` 并恢复，不要重新提交提示词：

```bash
pavo short-drama resume --conversation-id "CONVERSATION_ID"
```

查询持久状态或最终生成结果：

```bash
pavo short-drama status --conversation-id "CONVERSATION_ID"
pavo short-drama result --conversation-id "CONVERSATION_ID"
```

若最终 `results` 或 `assets[].result` 含有成功的 `url`，默认先返回 URL。仅在用户要求保存、导出或在桌面聊天中展示产物时下载：

```bash
pavo download-result \
  --url "RESULT_URL" \
  --output-path "LOCAL_OUTPUT_FILE"
```

也可在 `start`、`reply` 或 `resume` 中使用 `--download-dir "ABSOLUTE_DIRECTORY"`，让 CLI 为每一张成功图片和每一段成功视频写入绝对 `assets[].result.local_path`（同时兼容填入 `results[].local_path`）。

桌面聊天中需要展示角色图、场景图、关键帧或视频时，必须在该轮命令上传入 `--live-assets --download-dir`，并使用每个阶段独立的绝对目录，例如：

```bash
pavo short-drama reply \
  --conversation-id "CONVERSATION_ID" \
  --prompt "USER_REPLY" \
  --live-assets \
  --download-dir "C:\\pavo\\short-drama\\CONVERSATION_ID\\step-4-character-image"
```

启用 `--live-assets` 后，stdout 是 JSONL：每收到并下载完成一张图片或一段视频，就会先输出一条 `type: "asset_ready"` 记录。立即展示该记录中 `asset.result.local_path` 的资产，不要等本阶段的其他资产完成。最后一条 `type: "complete"` 的 `result` 包含完整汇总、`assistant_text` 和 `review`；仅在此时展示服务端的确认问题或要求用户选择下一步。不要只展示最后一个结果，也不要只返回远程 URL。

## 身份验证与失败处理

优先尝试所需命令；若 CLI 提示未登录，再向用户获取 PAVO 邮箱和密码并执行 `pavo login`。不得在回复、日志或生成文件中泄露密码、访问令牌或预签名上传 URL。

命令失败时报告 CLI 错误及已知的 `conversation_id`，然后停止；不要伪造结果、不要转用其他提供方、不要因短暂断线创建替代会话。
