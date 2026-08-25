---
name: media-generation
description: 使用 PAVO CLI 查询并并行下载当前登录人的图片/视频、查询 Pixa 当前支持的模型，并生成或编辑单张/多张图像与视频。适用于查看或下载个人图片和视频、文生图、参考图改图、图像美化、文生视频、图生视频、参考视频或音频驱动的视频创作；不用于短剧工作流。
---

# PAVO 基础图像与视频生成

使用 `pavo models` 动态读取服务端模型目录，再通过 `pavo generate image` 或 `pavo generate video` 完成创作。不要维护或引用静态模型表，不要绕过 CLI 直接调用 Pixa API，也不要改用其他图像或视频服务。

短剧、短漫剧及短剧多轮确认使用 `$short-drama`。

## 查询当前登录人的图片或视频

使用当前登录 Token 分页查询个人媒体库并并行下载查询结果。图片使用 `images`，视频使用 `videos`；不要使用 `short_drama_final`，短剧成片查询交给 `$short-drama`：

```bash
pavo visuals --category images --page 1 --page-size 5 \
  --download-dir "ABSOLUTE_WORKSPACE_PATH/pavo_outputs/visuals/images" \
  --download-concurrency 4
pavo visuals --category videos --page 1 --page-size 5 \
  --download-dir "ABSOLUTE_WORKSPACE_PATH/pavo_outputs/visuals/videos" \
  --download-concurrency 4
```

按用户要求传递页码和每页数量；未指定时沿用 `--page 1 --page-size 5`。查询命令必须下载资产，不要省略 `--download-dir`；使用默认 4 路并发，只有用户明确要求时才在 1–32 范围内调整 `--download-concurrency`。输出中的 `pagination` 是分页信息，`groups[].list[]` 是按日期分组的结果，`downloaded` 与 `failed` 是下载成功和失败数量。使用成功项的绝对 `local_path` 展示图片或视频，不要改用远程 URL；单项下载失败时继续展示其他成功资产，只告知该项 `download_error` 中的失败原因，不要把整个查询报告为失败。保留 `visual_id`、`resource_id`、`source`、`type`、`created_at` 和完整 `metadata`。

## 模型发现与选择

模型上下线、名称和订阅要求会变化。用户要求查看、比较或指定模型时，必须先查询对应目录：

```bash
pavo models --mode generate_image --online-only
pavo models --mode generate_video --online-only
```

输出中的 `code` 是 `--model` 唯一可用的值。`generate_video` 模型的 `modes` 表示支持的视频输入能力：`frames_to_video` 同时支持文生视频和首尾帧生视频，`omni_to_video` 支持图片、视频、音频等全能参考素材。

判断免费模型时只认 `tags[].code == "free"`，不要根据 `subscription_level == 0` 推断免费。`is_online: false` 的模型不可提交。生图未指定模型时沿用默认 `agnes-image`。纯文本视频或明确指定 1–2 张首尾帧图的视频未指定模型时可用默认 `agnes-video-new`。确定使用 `omni_to_video` 时先查询模型目录；若默认模型在线且支持该模式则继续使用，否则让用户从支持该模式的模型中选择。不要擅自切换到可能收费的模型。命令提交前仍会实时验证模型及视频模式。

短剧模式的模型目录也可分别查询：

```bash
pavo models --mode short_drama --type image --online-only
pavo models --mode short_drama --type video --online-only
```

用户已经明确要求生成或编辑时，选定符合输入能力的在线模型后直接提交 `pavo generate`。不执行积分估算，也不额外暂停；用户只要求查询、比较或设计方案时不要创建生成任务。

## 生成或编辑图像

文生图：

```bash
pavo generate image \
  --prompt "USER_PROMPT" \
  --model "agnes-image" \
  --ratio "auto" \
  --resolution "auto"
```

参考图编辑时为每张本地图或 HTTP(S) URL 重复传入 `--image`。本地文件由 CLI 自动上传，顶层 `images` 会作为参考素材发送；CLI 同时从原始提示词构造 Pixa 要求的 `creative_prompt_json`：

```bash
pavo generate image \
  --prompt "美颜一下，保持人物身份和背景不变" \
  --image "C:\\path\\portrait.jpg" \
  --ratio "9:16" \
  --resolution "SD"
```

图片参考最多 5 项。需要多张结果时用 `--count`，但具体批量上限由模型决定。

## 生成或编辑视频

### 判定视频模式

先根据用户对素材的用途判定模式，不要仅按图片数量选择：

- 用户明确要求“首帧”“尾帧”“从这个画面开始”“在两张图之间过渡”，或明确给出首尾帧顺序时，使用 `frames_to_video`。
- 用户要求参考人物身份、主体、角色、产品、服装、场景、构图或风格，并希望重新创作画面时，使用 `omni_to_video`。例如“参考图中的主体，在沙滩边跳舞”属于参考模式，不属于首帧模式。
- 参考视频、参考音频、多于 2 张图片或混合参考素材时，使用 `omni_to_video`。
- 如果无法确定图片是首尾帧还是仅作参考，必须在提交生成任务前询问用户：“这些图片是作为视频首/尾帧，还是只用于参考人物、风格或内容？”收到确认后显式传入对应的 `--video-mode`。

对带 1–2 张图片且用途不明确的任务不要使用 `--video-mode auto`，因为它会优先选择 `frames_to_video`。

`frames_to_video` 同时代表文生视频和首尾帧生视频。纯文生视频不传 `--image`，可直接使用当前默认 `agnes-video-new`：

```bash
pavo generate video \
  --prompt "USER_PROMPT" \
  --model "agnes-video-new" \
  --video-mode "frames_to_video" \
  --ratio "auto" \
  --resolution "auto" \
  --duration "auto" \
  --sound "auto"
```

首尾帧生视频使用 1 张首帧图或 2 张按首帧、尾帧顺序排列的图片：

```bash
pavo generate video \
  --prompt "让人物自然转身并看向镜头" \
  --model "agnes-video-new" \
  --video-mode "frames_to_video" \
  --image "C:\\path\\first-frame.png" \
  --duration "8" \
  --sound "false"
```

参考模式使用 `omni_to_video`。图片、视频或音频分别使用可重复的 `--image`、`--video`、`--audio`；参数既可为本地路径，也可为 HTTP(S) URL。`frames_to_video` 不接受 `--video` 或 `--audio`。

## 参数边界

CLI 接受的通用范围来自 Pixa 请求契约与当前能力配置：

- `--ratio`: `auto`、`1:1`、`4:3`、`3:4`、`16:9`、`9:16`、`3:2`、`2:3`、`21:9`、`4:5`、`5:4`。
- `--resolution`: `auto`、`SD`、`HD`、`UHD`。
- `--count`: `auto` 或 1–15；模型自身上限可能更低。
- `--duration`: `auto` 或 2–15 秒；仅视频，模型自身可用秒数可能更窄。
- `--sound`: `auto`、`true`、`false`；仅视频，部分模型只支持静音。
- `--video-mode`: `auto`、`omni_to_video`、`frames_to_video`；必须出现在所选模型的 `modes` 中。

动态模型目录不返回逐模型参数矩阵。因此，对非 Agnes 模型优先使用 `auto`，只有用户明确指定时才传具体值；若服务端报告组合不支持，如实返回错误并让用户调整。

CLI 对默认 Agnes 模型实施精确限制：

- `agnes-image`: 比例支持 `1:1`、`4:3`、`3:4`、`16:9`、`9:16`、`3:2`、`2:3`；分辨率仅 `SD`；每次最多 1 张。各字段也可为 `auto`。
- `agnes-video-new`: 比例支持 `9:16`、`16:9`、`1:1`、`4:3`、`3:4`、`3:2`、`2:3`、`21:9`；分辨率支持 `SD`、`HD`；时长 5–15 秒；支持声音。各字段也可为 `auto`。

## 会话、进度与结果

不传 `--conversation-id` 时，生成命令会自动创建会话；编辑同一上下文或继续已有会话时传入原 ID。始终保留最终 JSON 中的 `conversation_id`。流断开或返回已有任务在运行时，CLI 会自动恢复；进程被终止后运行：

```bash
pavo resume --conversation-id "CONVERSATION_ID"
```

默认返回远程 `url`。桌面聊天中需要展示结果，或用户明确要求保存时，传入绝对下载目录。所有由本 Skill 保存到本地的附件和生成产物统一放在当前工作区的 `pavo_outputs/` 下，并为每个任务使用独立子目录；只有用户明确指定其他输出路径时才使用该路径。

```bash
pavo generate image \
  --prompt "USER_PROMPT" \
  --live-assets \
  --download-dir "ABSOLUTE_WORKSPACE_PATH/pavo_outputs/TASK_NAME"
```

启用 `--live-assets` 后 stdout 是 JSONL：每个完成并下载的结果先输出 `type: "asset_ready"`，最后输出 `type: "complete"`。立即用 `asset.result.local_path` 展示图片或视频；不要等待其他结果，也不要把远程 URL 当作本地路径。未启用时 stdout 是单个最终 JSON。

## 身份验证与失败处理

优先尝试所需命令。若 CLI 提示未登录，获取用户手机号与国家码（未指定时用 `86`），运行 `pavo login send-code --country-code "COUNTRY_CODE" --phone-number "PHONE_NUMBER"`。发送成功后等待用户提供本次短信验证码，再运行 `pavo login --country-code "COUNTRY_CODE" --phone-number "PHONE_NUMBER"`；非交互终端仅在用户已明确提供验证码时增加 `--verification-code`。不要猜测或复用验证码，不得在回复、日志或文件中泄露验证码、访问令牌或预签名上传 URL。

命令失败时报告原始 CLI 错误和已知 `conversation_id`，然后停止。不要伪造结果，不要因断流创建替代会话。
