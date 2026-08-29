# Canvas 常见案例索引

每个文件是一个独立 case，包含真实用户请求、覆盖能力、前置条件、可直接执行的命令、需要读取的输出字段、验收标准和失败处理。先根据用户意图选择一个最接近的 case；只读取该 case 及它明确链接的参考文档，不要一次加载整个目录。

## 通用前置条件

1. 先直接执行 `pavo canvas status`。未登录时才进入手机号验证码登录；没有画布绑定时按用户意图选择已有项目或新建项目。
2. `MODEL_CODE` 必须来自本次实时 `pavo canvas model list` / `tool-specs` 输出，不从案例文本猜测。
3. 节点参数优先使用命令返回的 `node_key`；只有名称在当前画布唯一时才使用标题。
4. stdout 是单行业务 JSON，stderr 是进度。后续步骤从 stdout 读取 `node_key`、`run_node_key`、`plan_id`、`run_id`、`task_id`、`group_key` 与 `canvas_url`。
5. 案例中的搭图命令不代表生成授权。只有用户明确要求生成时才执行 `--run`、`canvas run` 或 `dag run`。

## workflow：完整任务闭环

| 用户场景 | Case |
|---|---|
| 选择/新建项目并绑定工作目录 | [workflow/workspace-setup.md](workflow/workspace-setup.md) |
| 上传本地参考图、建图片节点、连线并生成 | [workflow/upload-create-run.md](workflow/upload-create-run.md) |
| 用角色设定 shortcut 固定人物 | [workflow/character-setting-shortcut.md](workflow/character-setting-shortcut.md) |
| 用首尾帧 guide 创建视频模板 | [workflow/first-last-frame-guide.md](workflow/first-last-frame-guide.md) |
| 离线起草、lint 并编译高质量 Storyboard | [workflow/offline-storyboard-quality.md](workflow/offline-storyboard-quality.md) |
| 从剧情生成结构化分镜并搭建关键帧 | [workflow/storyboard-to-images.md](workflow/storyboard-to-images.md) |
| 从结构化分镜搭建视频节点并用 DAG 生成 | [workflow/storyboard-to-video-dag.md](workflow/storyboard-to-video-dag.md) |
| 对已有工作流做 DAG 计划、执行和检查 | [workflow/existing-graph-dag-run.md](workflow/existing-graph-dag-run.md) |
| 用 NDJSON 原子搭建一组节点/边/group | [workflow/ndjson-atomic-workflow.md](workflow/ndjson-atomic-workflow.md) |
| 查询、下载并保存历史产物 | [workflow/artifact-download.md](workflow/artifact-download.md) |

## node-types：单节点模式

| 节点类型 | Case |
|---|---|
| 文本节点 | [node-types/text.md](node-types/text.md) |
| 图片节点 | [node-types/image.md](node-types/image.md) |
| 视频节点 | [node-types/video.md](node-types/video.md) |
| 音频节点 | [node-types/audio.md](node-types/audio.md) |
| 上传节点 | [node-types/upload.md](node-types/upload.md) |
| CLI-only 结构化 Storyboard | [node-types/storyboard.md](node-types/storyboard.md) |

## failures：诊断与恢复

| 失败场景 | Case |
|---|---|
| `canvas validate --strict` 未通过 | [failures/validation-errors.md](failures/validation-errors.md) |
| 单节点异步任务、断线或回写失败 | [failures/generation-resume.md](failures/generation-resume.md) |
| DAG 返回 `replan_required` | [failures/dag-replan-required.md](failures/dag-replan-required.md) |

底层命令契约以 [`references/commands.md`](../references/commands.md) 为准；NDJSON/DAG 细节见 [`references/automation.md`](../references/automation.md)；prompt 与 storyboard 结构分别见 [`references/prompting.md`](../references/prompting.md) 和 [`references/storyboard.md`](../references/storyboard.md)。
