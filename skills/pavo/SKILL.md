---
name: pavo
description: 使用 PAVO CLI 上传聊天附件、创建或恢复会话，并获取 PAVO 设计生成结果。适用于由桌面代理处理的 PAVO 图像生成或聊天附件上传请求。
---

# PAVO

处理 PAVO 请求时，使用随附的 `pavo` CLI。支持的命令如下：

1. `pavo login`
2. `pavo upload`
3. `pavo conversation create`
4. `pavo stream`
5. `pavo resume`
6. `pavo conversation status` / `pavo conversation result`
7. `pavo download-result`

首次生成设计时，依次使用 `pavo login` → `pavo conversation create` → `pavo stream`。对于中断或已在运行的会话，请保留其 `conversation_id` 并使用 `pavo resume`；不要创建替代会话。不要改用直接的 `curl` 调用，也不要调用无关的图像或视频服务。

## 身份验证

如果可能已存有登录状态，请先尝试用户请求的会话命令。若 CLI 提示用户未登录，请在执行登录前获取用户的 PAVO 邮箱和密码。

优先使用交互式登录，以隐藏密码：

```bash
pavo login --email "USER_EMAIL"
```

对于非交互式桌面代理终端，仅在用户明确提供此次登录所需密码时使用 `--password`：

```bash
pavo login --email "USER_EMAIL" --password "USER_PASSWORD"
```

绝不可在回复、日志、摘要或生成的文件中重复密码或访问令牌。登录成功会输出用户信息，但绝不会输出访问令牌。

## 上传聊天附件

当用户明确要求上传本地 PAVO 聊天附件时，运行：

```bash
pavo upload --file "LOCAL_FILE_PATH"
```

从 stdout 读取 `public_url` 并返回该 URL。CLI 会处理需验证的预上传请求及无需验证的对象存储 PUT 请求；它绝不会输出临时签名上传 URL。

## 创建会话

原样使用用户的生成提示词：

```bash
pavo conversation create --prompt "USER_PROMPT"
```

从写入 stdout 的 JSON 中解析 `conversation_id`：

```json
{"conversation_id":"338562408542949376"}
```

CLI 会将标题编码为 PAVO 要求的文本分段格式，并将 `folder_id` 固定为空字符串、`kb_strict` 固定为 `false`。

## 流式接收生成结果

传入相同的提示词和返回的会话 ID。若用户提供本地参考文件，请为每个路径添加一个 `--file` 标志；CLI 会上传这些文件，并将生成的公开 URL 作为流式请求中的 `files` 发送：

```bash
pavo stream \
  --conversation-id "CONVERSATION_ID" \
  --prompt "USER_PROMPT" \
  --file "LOCAL_FILE_PATH"
```

CLI 会将 `mode` 固定为 `design`。它会持续读取流直到 `GenerationSuccess`，将进度写入 stderr，并向 stdout 输出一个最终 JSON 对象。没有附件时省略 `--file`；有多个本地附件时重复使用该标志。

如果服务返回 `070301`（已有活动流），或者发生暂时性的流传输失败，CLI 会切换到现有流并自动重连。不要仅因流客户端断开就创建第二个会话。

仅在诊断 PAVO 事件流时使用 `--raw`。原始事件会写入 stderr，从而保持 stdout 可由机器解析。

## 恢复中断的流

如果桌面环境停止了先前的 `pavo stream` 进程，请重新连接到现有任务，无需重新提交提示词或附件：

```bash
pavo resume --conversation-id "CONVERSATION_ID"
```

仅当当前进程已处理至该序号的事件时，才传入 `--from-seq LAST_SEQ`。否则省略该参数，以回放完整的已缓冲轮次。

## 查询状态和已完成的结果

短期的流回放缓冲区最终会过期。对于已完成的生成，请查询持久化的会话数据：

```bash
pavo conversation status --conversation-id "CONVERSATION_ID"
pavo conversation result --conversation-id "CONVERSATION_ID"
```

`status` 返回 `is_running`；`result` 返回最新持久化的生成结果。如果结果中包含成功的 URL，请先返回该 URL，再考虑新的生成。

## 下载生成结果

默认返回每个成功结果的 `url`；不要自动下载所有结果。仅在用户明确要求下载、保存或导出结果、要求本地路径，或获准的后续任务需要本地图像或视频文件时下载。在桌面聊天中展示生成的图像或视频就是这样的后续任务：桌面聊天渲染器需要绝对本地文件路径，可能无法渲染对象存储 URL。

```bash
pavo download-result \
  --url "RESULT_URL" \
  --output-path "LOCAL_OUTPUT_FILE"
```

`--output-path` 必须包含目标文件名。命令保存文件时返回 `downloaded`，安全复用本地文件时返回 `already_exist`。仅在用户要求替换现有本地文件时传入 `--force`。如果服务为结果提供 Unix 更新时间戳，请将其作为 `--updated-at` 传入；否则省略。

对于必须在桌面聊天中展示的 `stream` 或 `resume` 结果，优先使用内置的本地交接方式：

```bash
pavo stream \
  --conversation-id "CONVERSATION_ID" \
  --prompt "USER_PROMPT" \
  --download-dir "ABSOLUTE_TEMP_DIRECTORY"
```

`--download-dir` 会下载每个成功结果，并将其绝对 `local_path` 添加到 `results`。在回复的图像标记中使用该本地路径。若本地路径可用，不要将远程 `url` 嵌入为图像。如果生成已完成且仅有其 `url` 可用，请先调用 `pavo download-result` 再渲染它。

不要仅为了预览而下载待处理或失败的结果、空 URL 或 `thumbnail_url`。`base64` 结果不是 URL 下载；仅在增加专用的本地保存能力后再处理它。

## 返回结果

从最终 JSON 中读取 `results`。在桌面聊天中展示生成的视觉内容时，渲染其 `local_path` 而不是远程 `url`；可将 URL 保留为可选链接。仅在有用时使用 `thumbnail_url` 作为预览。用户需要技术细节时，同时保留报告的 `width`、`height`、`ratio` 和 `mimetype`。发生下载时，同时提供返回的本地输出路径和结果 URL。

如果登录、创建会话、流式传输、恢复或查询结果失败，请报告 CLI 错误并停止。不要伪造结果，也不要继续使用其他提供方。
