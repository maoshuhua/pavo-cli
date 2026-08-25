# PAVO CLI

PAVO 桌面端创作 CLI。项目提供十一项业务能力：

1. 通过手机号短信验证码登录。
2. 上传聊天附件并获取可展示的公共地址。
3. 将已生成的图片或视频 URL 下载为本地文件。
4. 在客户端断线或重复进入会话时恢复已有生成流。
5. 查询会话运行状态，并从持久历史读取最终生成结果。
6. 分页查询并并行下载当前登录人的图片与视频。
7. 分页查询并并行下载当前登录人的已完成短剧。
8. 创建、续写、恢复和查询多轮短剧会话。
9. 按短剧、生图或生视频模式动态查询当前支持的模型。
10. 通过 Pixa 创意流生成或编辑基础图像与视频。
11. 创建、绑定和操作 Pixa 无限画布项目、节点、连线与生成任务。

仓库包含 `skills/canvas/`、`skills/media-generation/` 与 `skills/short-drama/`。npm 安装和更新时会把这三份 PAVO Skill 安装到桌面端的全局技能目录。

## 安装

需要 Node.js 16 或更高版本。发布后可通过 npm 安装预编译的 macOS、Linux 和 Windows 二进制，并把 PAVO Skill 注册到本机检测到的桌面端：

```bash
npx @pavo-dev/cli@latest install
pavo --version
```

在 npm Registry 发布前，也可以直接通过 GitHub Release 安装：

```bash
npx -y github:maoshuhua/pavo-cli#v0.1.4 install github:maoshuhua/pavo-cli#v0.1.4
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
POST https://api-pavo-test.pavo-ai.cn/api/v1/user/code/send
POST https://api-pavo-test.pavo-ai.cn/api/v1/user/auth/phone-otp
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
pavo visuals --category images --page 1 --page-size 5 \
  --download-dir "C:\workspace\pavo_outputs\visuals\images" \
  --download-concurrency 4
pavo visuals --category videos --page 1 --page-size 5 \
  --download-dir "C:\workspace\pavo_outputs\visuals\videos" \
  --download-concurrency 4
```

已完成短剧通过短剧命令组查询；CLI 会固定使用接口类别 `short_drama_final`：

```bash
pavo short-drama list --page 1 --page-size 5 \
  --download-dir "C:\workspace\pavo_outputs\visuals\short_drama_final" \
  --download-concurrency 4
```

三种查询都使用当前登录 Token 调用：

```http
GET /api/v1/visuals?page_size=5&category=images&page=1
Authorization: Bearer <access_token>
```

查询完成后，CLI 使用受控并发下载每项资产，默认并发数为 4，可通过 `--download-concurrency` 在 1–32 范围内调整。不指定 `--download-dir` 时，默认保存到当前工作区的 `pavo_outputs/visuals/<category>/`。输出保留服务端的 `pagination`、按日期组织的 `groups[].list[]` 与完整 `metadata`，并返回 `downloaded`、`failed` 计数。下载成功的条目写入绝对 `local_path`；单项失败只在该条目写入 `download_error` 原因，不中断其他资产下载，也不影响成功资产展示。

## Pixa 无限画布

列出或创建画布项目，并把当前工作区绑定到目标画布：

```bash
pavo canvas project list
pavo canvas project create --title "产品视觉方案" --use
pavo canvas status
```

绑定写入当前工作区的 `.pavo/canvas.json`，只保存 `project_uuid`、`canvas_uuid` 和客户端 session，不包含 Access Token。绑定后可省略每条命令的 `--project` / `--canvas`。

`project list/create/duplicate/show`、`use/status`、单节点运行和 DAG 输出会附带 `canvas_url`。测试 API 默认对应 `https://app-test.pavo-ai.cn`，网页路由使用数值 `project_id`，并把 `canvas_uuid`、`project_uuid` 放在查询参数中。

节点与连线使用前端相同的 batch 数据契约。CLI 每次写入前读取最新画布版本；遇到明确的版本冲突只重新读取并重放一次，同时保留节点 `data` 中 CLI 不认识的新字段：

```bash
pavo canvas node create \
  --type image \
  --name "主视觉" \
  --prompt "清晨海边的产品主视觉" \
  --model "MODEL_CODE"
pavo canvas upload --file "C:\path\reference.png" --name "参考图"
pavo canvas edge add --source "参考图" --target "主视觉"
pavo canvas node list
pavo canvas edge list
```

可按前端坐标语义创建或解除 group。Codex/脚本批量搭图时，`apply --stdin` 接收一行一个 JSON object 的 NDJSON，先完整校验再用一次 `nodes/batch` 原子提交；`--dry-run` 不修改画布：

```bash
pavo canvas group create "参考图" "主视觉" --name "主视觉组"
pavo canvas group ungroup "主视觉组" --yes
pavo canvas apply --stdin --dry-run < workflow.ndjson
pavo canvas apply --stdin < workflow.ndjson
```

NDJSON 只包含节点、连线和分组等图结构操作，不混入上传、生成或 artifact 删除。流中存在删除或解组时，实际提交必须传 `--yes`。

图片、视频或音频节点传 `--model` 时，CLI 会实时验证模型是否存在、在线且对当前账号可用，并按该模型 constraints 写入前端运行所需的 `modeType`、默认比例/分辨率、图片数量或视频时长。

画布模型和文本工具动态来自 Pixa：

```bash
pavo canvas model list --scene canvas_image
pavo canvas model list --scene canvas_video
pavo canvas model list --scene canvas_audio
pavo canvas tool-specs
```

运行节点会按前端约定回写节点执行态和最终 URL/文本结果，确保网页刷新后仍能恢复和展示。命令默认等待生成终态；传 `--download` 后，成功资源默认保存到当前工作区 `pavo_outputs/canvas/<task_id>/`，也可用 `--output-dir` 指定目录。输出中的每个成功结果增加绝对 `local_path`，单项下载失败只增加 `download_error`，不会改变生成任务的成功状态。异步提交用 `--wait=false`，再按返回的 `task_id` 查询、等待或取消；异步模式不能同时下载。生成提交不会自动重试，避免重复创建任务：

```bash
pavo canvas run "主视觉" --download
pavo canvas run "主视觉" --download --output-dir "C:\workspace\pavo_outputs\canvas\task-name"
pavo canvas task status "TASK_ID"
pavo canvas task wait "TASK_ID" --timeout 30m
pavo canvas task cancel "TASK_ID"
```

多个生成节点存在依赖时，使用 DAG 计划统一做环检测、拓扑排序并固定节点参数摘要。`plan` 不创建任务；执行时引用同一个 `plan_id`：

```bash
pavo canvas dag plan --group "主视觉组"
pavo canvas dag plan --target "最终视频"
pavo canvas dag plan --all
pavo canvas dag run --plan "PLAN_ID" --max-parallel 4 --download
pavo canvas dag status "RUN_ID"
pavo canvas dag resume "RUN_ID"
```

DAG 发现依赖环会直接报告环路径并停止。执行前会重新校验图和参数，变化时返回 `replan_required`。同层节点受 `--max-parallel` 控制并行；上游失败只跳过其后代，独立分支继续。`.pavo/canvas-plans/` 与 `.pavo/canvas-runs/` 保存本地计划和恢复清单，不含 Token；恢复复用每个节点原始的幂等 request ID。

画布历史产物可按日期组查询和下载，也可以按节点资源保存到“我的资产”：

```bash
pavo canvas artifact list --category videos --page 1 --page-size 5
pavo canvas artifact list --download-dir "C:\workspace\canvas-artifacts"
pavo canvas artifact save "最终视频" --resource-index 0 --name "最终成片"
pavo canvas artifact delete "ARTIFACT_UUID" --yes
```

Artifact 列表的 `page_size` 与 `pagination.total` 都按“有产物的日期组”计算。删除是幂等软删历史记录，不删除节点当前资源、已保存资产或对象存储；批量最多 100 个 UUID。

删除项目、节点、连线、group 或 artifact 历史记录需要显式 `--yes`。完整命令、NDJSON 与节点 data 约定见 `skills/canvas/references/`。

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

无限画布技能位于 `skills/canvas/SKILL.md`，基础图像/视频技能位于 `skills/media-generation/SKILL.md`，短剧技能位于 `skills/short-drama/SKILL.md`。

`canvas` 使用 `pavo canvas` 管理画布项目和工作区绑定，并按前端兼容格式操作节点、连线、group、NDJSON 批量变更、DAG 生成和历史产物。单节点按用户明确要求直接运行；多节点 DAG 由 `canvas dag plan` 固定拓扑、节点参数和 `plan_hash` 后执行。Skill 会主动返回 CLI 输出的 `canvas_url`。它不会绕过 CLI 直接请求 Pixa，也不会把普通图片/视频生成混入画布任务。

`media-generation` 使用 `pavo visuals` 查询当前登录人的图片或视频，使用 `pavo models` 查询实时模型目录，并在用户明确要求生成时调用 `pavo generate image` 或 `pavo generate video`。本地参考素材由命令自动上传；生成任务复用统一的流恢复、结果下载和实时资产输出能力。

用户明确要求上传聊天附件时，Skill 会单独调用 `pavo upload --file <本地路径>` 并返回其 `public_url`。媒体生成命令通过可重复的 `--image`、`--video`、`--audio` 接收本地素材并自动上传；短剧命令通过可重复的 `--file` 绑定附件。

`short-drama` 使用 `pavo short-drama list` 查询当前登录人的短剧成片，使用 `pavo short-drama start` 创建首轮短剧会话，并用 `pavo short-drama reply` 在相同 `conversation_id` 上继续多轮对话。纯阶段推进不因计费信息暂停；涉及剧情、风格、角色或镜头等创作选择时仍转交用户决定。

## 配置

| 环境变量 | 说明 |
| --- | --- |
| `PAVO_API_BASE_URL` | API 基础地址，默认 `https://api-pavo-test.pavo-ai.cn` |
| `PAVO_APP_BASE_URL` | 网页应用基础地址，测试 API 默认映射为 `https://app-test.pavo-ai.cn`，生产 API 映射为 `https://app.pavo-ai.cn` |
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
