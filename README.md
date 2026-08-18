# PAVO CLI

PAVO 桌面端创作 CLI。项目提供十项业务能力：

1. 通过手机号短信验证码登录。
2. 上传聊天附件并获取可展示的公共地址。
3. 将已生成的图片或视频 URL 下载为本地文件。
4. 在客户端断线或重复进入会话时恢复已有生成流。
5. 查询会话运行状态，并从持久历史读取最终生成结果。
6. 分页查询当前登录人的图片与视频。
7. 分页查询当前登录人的已完成短剧。
8. 创建、续写、恢复和查询多轮短剧会话。
9. 按短剧、生图或生视频模式动态查询当前支持的模型。
10. 通过 Pixa 创意流生成或编辑基础图像与视频。

仓库包含 `skills/media-generation/` 与 `skills/short-drama/`。npm 安装和更新时会把这两份 PAVO Skill 安装到桌面端的全局技能目录。

## 安装

需要 Node.js 16 或更高版本。发布后可通过 npm 安装预编译的 macOS、Linux 和 Windows 二进制，并把 PAVO Skill 注册到本机检测到的桌面端：

```bash
npx @pavo-dev/cli@latest install
pavo --version
```

在 npm Registry 发布前，也可以直接通过 GitHub Release 安装：

```bash
npx -y github:maoshuhua/pavo-cli#v0.1.3 install github:maoshuhua/pavo-cli#v0.1.3
pavo --version
```

安装完成后重启或新建一个桌面端会话，使新 Skill 被重新加载。

也可以在源码目录构建：

```bash
go build -o pavo .
```

## 登录

先发送短信验证码。`--country-code` 默认是 `86`，传值时不带 `+`：

```bash
pavo login send-code \
  --country-code "86" \
  --phone-number "PHONE_NUMBER"
```

收到验证码后登录。省略 `--verification-code` 时，CLI 会隐藏读取输入，避免写入 Shell 历史：

```bash
pavo login \
  --country-code "86" \
  --phone-number "PHONE_NUMBER"
```

桌面端的非交互场景，仅在用户已明确提供本次收到的验证码时使用 `--verification-code`，也可通过 `PAVO_VERIFICATION_CODE` 提供。登录成功后，Access Token 保存在系统用户配置目录的 `pavo/config.json` 中；验证码、Token 和预签名 URL 不会出现在命令输出中。

```bash
pavo login \
  --country-code "86" \
  --phone-number "PHONE_NUMBER" \
  --verification-code "VERIFICATION_CODE"
```

验证码与登录接口：

```http
POST https://api.pavo-ai.cn/api/v1/user/code/send
POST https://api.pavo-ai.cn/api/v1/user/auth/phone-otp
```

CLI 分别发送以下请求体：

```json
{"country_code":"86","phone_number":"PHONE_NUMBER","scene":"phone_auth"}
```

```json
{"country_code":"86","phone_number":"PHONE_NUMBER","verification_code":"VERIFICATION_CODE"}
```

## 查询当前登录人的图片、视频与短剧

图片和视频通过 `visuals` 命令分页查询：

```bash
pavo visuals --category images --page 1 --page-size 5
pavo visuals --category videos --page 1 --page-size 5
```

已完成短剧通过短剧命令组查询；CLI 会固定使用接口类别 `short_drama_final`：

```bash
pavo short-drama list --page 1 --page-size 5
```

三种查询都使用当前登录 Token 调用：

```http
GET /api/v1/visuals?page_size=5&category=images&page=1
Authorization: Bearer <access_token>
```

输出保留服务端的 `pagination` 与按日期组织的 `groups[].list[]`，并完整透传每条结果的 `metadata`。图片或视频地址可能位于条目顶层，也可能位于 `metadata.url`、`metadata.thumbnail_url` 或 `metadata.original_url`。

## 查询支持的模型

模型目录直接来自 Pixa 的实时配置，不随 CLI 版本硬编码。`--mode` 支持 `short_drama`、`generate_image`、`generate_video`：

```bash
pavo models --mode generate_image --online-only
pavo models --mode generate_video --online-only
pavo models --mode short_drama --type image --online-only
pavo models --mode short_drama --type video --online-only
```

接口：

```http
GET /api/v1/pixa/mode_support_models?mode_code=generate_image
Authorization: Bearer <access_token>
```

输出保留模型的 `code`、`name`、`is_online`、`tags`、订阅信息，以及视频模型的 `modes`。其中 `tags[].code == "free"` 才表示免费；不能用 `subscription_level == 0` 代替。`generate_video` 的 `frames_to_video` 同时表示支持文生视频和首尾帧生视频，`omni_to_video` 表示支持图片、视频、音频等全能参考素材。

## 基础图像与视频生成

基础文生图会自动创建 conversation、实时验证模型，并以 `mode: "generate_image"` 发起创意流：

```bash
pavo generate image \
  --prompt "美女图" \
  --model "agnes-image" \
  --ratio "auto" \
  --resolution "SD"
```

编辑参考图时可重复传入本地路径或 HTTP(S) URL。CLI 会自动上传本地素材，并构造 Pixa 所需的 `creative_prompt_json`：

```bash
pavo generate image \
  --conversation-id "346482729455452160" \
  --prompt "美颜下" \
  --image "C:\\path\\portrait.jpg" \
  --ratio "9:16" \
  --resolution "SD"
```

请求体的关键字段形如：

```json
{
  "conversation_id": 346482729455452160,
  "prompt": "美颜下",
  "mode": "generate_image",
  "model": "agnes-image",
  "ratio": "9:16",
  "resolution": "SD",
  "creative_prompt_json": "[{\"type\":\"text\",\"content\":\"美颜下\"}]",
  "images": [{"url": "https://example.test/portrait.jpg"}]
}
```

视频使用 `generate video`。默认模型 `agnes-video-new` 当前属于 `frames_to_video`，既可不传图片进行文生视频，也可传 1 张首帧图或 2 张首尾帧图：

```bash
pavo generate video \
  --prompt "让人物自然转身并看向镜头" \
  --model "agnes-video-new" \
  --video-mode "frames_to_video" \
  --image "C:\\path\\first-frame.png" \
  --ratio "9:16" \
  --resolution "HD" \
  --duration "8" \
  --sound "false"
```

参考视频、参考音频、多于 2 张图片或混合参考任务，应先从 `pavo models --mode generate_video --online-only` 的结果中选择 `modes` 含 `omni_to_video` 的模型，并传 `--video-mode omni_to_video`。图片、视频和音频参考素材分别使用可重复的 `--image`、`--video`、`--audio`；`--video-mode auto` 对纯文本和 1–2 张首尾帧图片优先选择 `frames_to_video`，其余参考素材选择 `omni_to_video`。

通用参数支持 `ratio=auto`、`resolution=auto`、`count=auto`、`duration=auto` 和 `sound=auto`，以便由服务端按模型能力选择。CLI 对 Agnes 默认模型实施 agnes_core 能力限制：`agnes-image` 仅支持 SD、每次 1 张；`agnes-video-new` 支持 SD/HD 和 5–15 秒。其他模型的精确组合以服务端最新配置为准。

两个生成命令都支持 `--conversation-id` 继续已有会话、`--download-dir` 下载结果，以及 `--live-assets` 逐个输出完成的本地资产。仓库内保存的附件和生成产物统一放在 `pavo_outputs/<任务子目录>/`；用户显式指定其他输出路径时以用户路径为准。断流会沿用现有 `resume` 机制，不会重复提交任务。

## 短剧（多轮会话）

短剧通过同一个聊天流接口发送 `mode: "short_drama"`，并在 `extra_context.agent_params` 中指定默认的 `agnes-image` 与 `agnes-video-new` 模型。首次调用会创建会话并发送首轮需求：

```bash
pavo short-drama start --prompt "制作一支南京宣传片"
```

结果中的 `conversation_id` 是后续所有短剧轮次的唯一标识。服务端提问、要求确认或需要调整时，使用同一个 ID 继续，不要创建新会话：

```bash
pavo short-drama reply \
  --conversation-id "340407156788563968" \
  --prompt "改成水墨动画风格"
```

需要本地参考文件时，在 `start` 或 `reply` 上重复传入 `--file`；CLI 会先上传文件再绑定到该轮请求。短剧流中断时恢复现有轮次，避免重复提交：

```bash
pavo short-drama resume --conversation-id "340407156788563968"
pavo short-drama status --conversation-id "340407156788563968"
pavo short-drama result --conversation-id "340407156788563968"
```

`start` 与 `reply` 支持 `--image-model-code`、`--video-model-code` 覆盖默认模型；`start`、`reply` 与 `resume` 都支持 `--download-dir` 为成功结果写入绝对本地路径。

桌面端需要逐张展示分镜图或逐段展示分镜视频时，给 `start`、`reply` 或 `resume` 增加 `--live-assets --download-dir <绝对目录>`。此时 stdout 改为 JSONL：每个已下载的产物先输出 `{"type":"asset_ready","asset":...}`，最后以 `{"type":"complete","result":...}` 收束本轮；未启用该选项时仍保持原有单个最终 JSON 输出。

## 恢复长任务

同一个 `conversation_id` 同时只能有一条生成流。生图、生视频或短剧命令收到已有任务运行错误 `070301` 后，会自动改接恢复流，不会重复提交生成。客户端在瞬时断线后也会从最后收到的事件序号自动恢复。

也可以手动恢复被桌面环境停止的流：

```bash
pavo resume --conversation-id "338562408542949376"
```

如果已经处理过部分事件，传入最大的 `seq` 可避免重复事件：

```bash
pavo resume --conversation-id "338562408542949376" --from-seq 42
```

生成结束后，服务端的短期回放缓存会过期。此时从持久会话历史取回产物：

```bash
pavo conversation status --conversation-id "338562408542949376"
pavo conversation result --conversation-id "338562408542949376"
```

`status` 返回是否仍在运行；`result` 返回最近一轮持久化的图片或视频结果 URL。

## 下载生成结果

生成命令默认只返回结果 URL，不会写入本地磁盘。当用户明确要求下载、保存、导出，或后续步骤需要使用本地图片/视频文件时，再调用下载命令：

```bash
pavo download-result \
  --url "https://example.test/image.jpg" \
  --output-path "C:\\workspace\\pavo_outputs\\task-name\\image.jpg"
```

目标路径必须包含文件名。下载会先写入同目录临时文件，成功后再替换目标文件，避免生成半个文件。默认已有同名文件会跳过：

```json
{
  "output_path": "C:\\workspace\\pavo_outputs\\task-name\\image.jpg",
  "already_exist": ["C:\\workspace\\pavo_outputs\\task-name\\image.jpg"]
}
```

如服务端提供了资源更新时间，可传入 `--updated-at <Unix 秒级时间戳>`：只有本地文件较旧时才更新。用 `--force` 可无条件覆盖已有文件。

下载使用结果 URL 的公开访问能力，不会向对象存储或 CDN 发送 PAVO Access Token。

如需在生成命令结束后直接交付给需要本地绝对路径的桌面界面，可传入 `--download-dir`。CLI 会下载每个成功结果，并在对应 JSON `results` 项中填入 `local_path`：

```bash
pavo generate image \
  --prompt "USER_PROMPT" \
  --download-dir "C:\\workspace\\pavo_outputs\\task-name"
```

## 上传聊天附件

```bash
pavo upload --file "C:\\path\\to\\Image1.jpg"
```

CLI 先向 PAVO API 获取预签名上传地址，再直接 PUT 文件到对象存储。预上传请求使用当前登录 Token；对象存储直传不携带 Token。成功后仅输出用于展示或后续业务引用的公共地址，不输出短期有效的签名上传地址：

```json
{
  "public_url": "https://cos-aigc-default-test.kiwiar.com/pixa/chat_attachment/.../Image1.jpg",
  "content_type": "image/jpeg",
  "filename": "Image1.jpg"
}
```

## 桌面端 Skills

基础图像/视频技能位于 `skills/media-generation/SKILL.md`，短剧技能位于 `skills/short-drama/SKILL.md`。

`media-generation` 使用 `pavo visuals` 查询当前登录人的图片或视频，使用 `pavo models` 查询实时模型目录，再调用 `pavo generate image` 或 `pavo generate video`。本地参考素材由命令自动上传；生成任务复用统一的流恢复、结果下载和实时资产输出能力。

用户明确要求上传聊天附件时，Skill 会单独调用 `pavo upload --file <本地路径>` 并返回其 `public_url`。媒体生成命令通过可重复的 `--image`、`--video`、`--audio` 接收本地素材并自动上传；短剧命令通过可重复的 `--file` 绑定附件。

`short-drama` 使用 `pavo short-drama list` 查询当前登录人的短剧成片，使用 `pavo short-drama start` 创建首轮短剧会话，并用 `pavo short-drama reply` 在相同 `conversation_id` 上继续多轮对话。它只转交用户原始创作需求与服务端问题，不替用户决定剧情、风格、角色或镜头。

## 配置

| 环境变量 | 说明 |
| --- | --- |
| `PAVO_API_BASE_URL` | API 基础地址，默认 `https://api.pavo-ai.cn` |
| `PAVO_ACCESS_TOKEN` | 临时覆盖本地保存的 Access Token |
| `PAVO_VERIFICATION_CODE` | 非交互手机号登录使用的本次短信验证码 |
| `PAVO_HTTP_TIMEOUT` | HTTP 和生成流超时，默认 `10m` |
| `PAVO_CONFIG_FILE` | 覆盖登录信息文件路径，主要用于测试 |
| `PAVO_CLI_DISABLE_UPDATE_CHECK=1` | 关闭 npm 版本检查 |

## 更新和发布

```bash
pavo update
```

更新命令通过 npm 更新 PAVO CLI，并重新安装全部 PAVO Skill。GoReleaser 构建以下平台：

- macOS amd64/arm64
- Linux amd64/arm64
- Windows amd64/arm64

发布前需要确保 `package.json`、Go module、GitHub 仓库和 npm scope 均指向实际的 PAVO 发布位置。

## 开发验证

```bash
npm test
```

该命令执行 JavaScript 安装链路测试、Go 单元测试和 `go vet ./...`。
