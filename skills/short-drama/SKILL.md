---
name: short-drama
description: 使用 PAVO CLI 查询当前登录人的短剧成片，以及创建、续写、修改和恢复多轮短剧创作会话。适用于查看个人短剧、查询已生成短剧、创作短剧、写短剧剧本、继续某个短剧、回答短剧创作问题、调整短剧风格或获取短剧生成结果时。短剧必须通过 pavo short-drama 命令完成。
---

# PAVO 短剧

使用 `pavo short-drama` 处理短剧创作。短剧是服务端维护状态的多轮会话：首次创建会话，后续补充、选择、确认与修改都复用同一个 `conversation_id`。不要改用其他生成命令，不要调用其他图像或视频服务，也不要自行代写、扩写或重组用户的短剧需求。

## 命令

1. `pavo short-drama start`
2. `pavo short-drama reply`
3. `pavo short-drama resume`
4. `pavo short-drama status`
5. `pavo short-drama result`
6. `pavo short-drama list`
7. `pavo download-result`

## 查询当前登录人的短剧

分页查询当前登录人的已完成短剧成片。此命令固定使用服务端类别 `short_drama_final`：

```bash
pavo short-drama list --page 1 --page-size 5
```

按用户要求传递页码和每页数量；未指定时沿用 `--page 1 --page-size 5`。输出中的 `pagination` 是分页信息，`groups[].list[]` 是按日期分组的短剧。展示时优先读取条目顶层的 `url`、`thumbnail_url`；若为空，则读取 `metadata.url`、`metadata.thumbnail_url` 或 `metadata.original_url`。这用于跨会话浏览个人短剧；已知 `conversation_id` 且需要读取某个会话的持久结果时，仍使用 `pavo short-drama result`。

## 首次创作

原样传递用户的短剧需求。需要本地参考文件时，为每个文件增加一个 `--file`。命令会创建会话，并以 `mode: "short_drama"`、`extra_context.agent_params.image_model_code: "agnes-image"` 和 `video_model_code: "agnes-video-new"` 发起首轮请求：

```bash
pavo short-drama start --prompt "USER_PROMPT"
```

默认从 stdout 的最终 JSON 记录并向用户返回 `conversation_id`。优先读取 `assistant_text`（完整服务端文本）、`assistant_messages` 和 `review`，将服务端给出的剧本、角色、场景或确认详情完整展示给用户；不要只做一句概括。过程事件会写入 stderr，仅用于诊断。

## 模型选择

默认使用 `agnes-image` 与 `agnes-video-new`。用户未要求选择或切换模型时，不要主动展示模型表，也不要自行替换默认值；服务端会在提交时验证可用性。

用户明确要求选择、比较或切换模型时，不得使用静态模型表。先动态查询当前已上线的短剧模型：

```bash
pavo models --mode short_drama --type image --online-only
pavo models --mode short_drama --type video --online-only
```

将查询结果展示给用户，让用户分别选择一个图像模型和一个视频模型，不要代替用户决定。只使用返回的 `code`；判断免费模型时只认 `tags[].code == "free"`，不要用 `subscription_level == 0` 推断免费。服务端若返回模型无权限或不支持，如实报告并等待用户重新选择。

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

每次 `reply` 仍会携带短剧模式和默认的 Agnes 图片、视频模型。不要为后续轮次创建新会话；始终使用 `reply` 继续。

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
  --output-path "ABSOLUTE_WORKSPACE_PATH/pavo_outputs/short-drama/CONVERSATION_ID/RESULT_FILE"
```

所有由本 Skill 保存到本地的附件和生成产物统一放在当前工作区的 `pavo_outputs/` 下，并按短剧会话和阶段建立子目录；只有用户明确指定其他输出路径时才使用该路径。也可在 `start`、`reply` 或 `resume` 中使用 `--download-dir`，让 CLI 为每一张成功图片和每一段成功视频写入绝对 `assets[].result.local_path`（同时兼容填入 `results[].local_path`）。

桌面聊天中需要展示角色图、场景图、关键帧或视频时，必须在该轮命令上传入 `--live-assets --download-dir`，并使用每个阶段独立的绝对目录，例如：

```bash
pavo short-drama reply \
  --conversation-id "CONVERSATION_ID" \
  --prompt "USER_REPLY" \
  --live-assets \
  --download-dir "ABSOLUTE_WORKSPACE_PATH/pavo_outputs/short-drama/CONVERSATION_ID/step-4-character-image"
```

启用 `--live-assets` 后，stdout 是 JSONL：每收到并下载完成一张图片或一段视频，就会先输出一条 `type: "asset_ready"` 记录。立即展示该记录中 `asset.result.local_path` 的资产，不要等本阶段的其他资产完成。最后一条 `type: "complete"` 的 `result` 包含完整汇总、`assistant_text` 和 `review`；仅在此时展示服务端的确认问题或要求用户选择下一步。不要只展示最后一个结果，也不要只返回远程 URL。

## 身份验证与失败处理

优先尝试所需命令。若 CLI 提示未登录，获取用户手机号与国家码（未指定时用 `86`），运行 `pavo login send-code --country-code "COUNTRY_CODE" --phone-number "PHONE_NUMBER"`。发送成功后等待用户提供本次短信验证码，再运行 `pavo login --country-code "COUNTRY_CODE" --phone-number "PHONE_NUMBER"`；非交互终端仅在用户已明确提供验证码时增加 `--verification-code`。不要猜测或复用验证码，不得在回复、日志或生成文件中泄露验证码、访问令牌或预签名上传 URL。

命令失败时报告 CLI 错误及已知的 `conversation_id`，然后停止；不要伪造结果、不要转用其他提供方、不要因短暂断线创建替代会话。
