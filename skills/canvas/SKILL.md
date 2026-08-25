---
name: canvas
description: 使用 PAVO CLI 创建、绑定和修改 Pixa 无限画布，操作节点、连线、分组、批量图变更、DAG 生成及历史产物。用户要求在 PAVO/Pixa 画布中编排或继续创作时使用；不用于脱离画布的普通图片、视频或短剧生成。
---

# PAVO 无限画布

只通过 `pavo canvas` 命令操作画布，不直接请求 Pixa API。CLI 的 stdout 是业务 JSON，生成进度写到 stderr；解析时不要混用两者。

## 选择项目

先运行 `pavo canvas status`。若当前工作区没有绑定：

- 用户给出了项目 UUID：运行 `pavo canvas use --project "PROJECT_UUID"`。
- 用户要使用已有项目但未指出具体项目：运行 `pavo canvas project list`，让用户从可能的多个项目中确认；只有唯一且语义明确时可直接选择。
- 用户明确要求新建：运行 `pavo canvas project create --title "TITLE" --use`。

绑定保存在工作区 `.pavo/canvas.json`，其中没有令牌。始终用 `use` / `unuse` 管理，不手工编辑。后续命令默认读取绑定；临时跨项目操作才显式传 `--project` / `--canvas`。

`status`、项目命令、单节点运行和 DAG 输出中的 `canvas_url` 是对应画布的网页地址。涉及某张画布时，最终回复主动附上这个可点击链接；不要只返回 UUID，也不要用 `project_uuid` 自行替代 URL 路径中的数值 `project_id`。

## 操作流程

变更前用 `pavo canvas node list` 和 `pavo canvas edge list` 读取当前图。节点参数既可用精确 `node_key`，也可用精确标题；标题重名时报错后改用 `node_key`，不要猜测。

创建生成节点前查询实时配置：图片、视频、音频分别使用 `pavo canvas model list --scene canvas_image|canvas_video|canvas_audio`；文本工具或节点模板使用 `pavo canvas tool-specs`。模型 code、约束和在线状态会变化，不维护静态模型表。

常见闭环：

1. 用 `node create` 建文本、图片、视频或音频节点；本地参考素材用 `canvas upload`，该命令会同时创建 upload 节点。
2. 用 `edge add --source ... --target ...` 建立依赖。CLI 会同步节点 data 中的 `source` / `target`。
3. 再次读取目标节点，确认 prompt、model 和引用关系。
4. 用户已经明确要求生成时，用 `pavo canvas run NODE --download` 创建任务、等待终态并把成功资源下载到本地；结果使用 `task.task_result.results[].local_path` 展示或继续处理。用户指定目录时增加 `--output-dir "ABSOLUTE_PATH"`。只需异步提交时传 `--wait=false`，随后用 `pavo canvas task status` / `pavo canvas task wait`。

两个以上且存在依赖的生成节点优先使用 DAG：先运行 `pavo canvas dag plan --group GROUP|--target NODE|--all`。计划会拒绝依赖环，并把拓扑、节点参数摘要和 `plan_hash` 写入 `.pavo/canvas-plans/`。用户已经明确要求执行该范围时，直接用 `pavo canvas dag run --plan PLAN_ID --download` 执行，不做积分估算或额外暂停。中断后先用 `pavo canvas dag status RUN_ID` 刷新状态，再用 `pavo canvas dag resume RUN_ID`；恢复会复用原始 request ID，不自行重建计划或重复运行已成功节点。

整理已有节点时用 `pavo canvas group create NODE NODE...`；解组会删除 group 节点，只有用户明确要求时才运行 `group ungroup GROUP --yes`。Codex 或脚本需要一次搭建整张图时，可用 `pavo canvas apply --stdin --dry-run` 校验 NDJSON，再去掉 `--dry-run` 原子提交。NDJSON 只承载节点、连线和分组等图结构变更，不包含上传、生成或 artifact 删除。

历史产物用 `pavo canvas artifact list` 查询；跨宿主使用时增加 `--download-dir "ABSOLUTE_PATH"` 并使用返回的 `local_path`。保存到“我的资产”使用 `artifact save NODE --resource-index N`。历史记录删除只在用户明确要求时执行 `artifact delete UUID... --yes`；它不会删除节点当前资源、已保存资产或对象存储文件。

执行具体命令前按需阅读 [references/commands.md](references/commands.md)。需要批量搭图或执行 DAG 时阅读 [references/automation.md](references/automation.md)；需要传自定义 `--data`、构造提示词片段或理解节点字段时，再阅读 [references/node-data.md](references/node-data.md)。

## 边界与失败处理

- 普通画布批量写会在明确的版本冲突后重新读取并重放一次；不要在外层循环重复命令。
- `.pavo/canvas-plans/` 与 `.pavo/canvas-runs/` 是 CLI 的恢复清单，不含令牌；不要手工修改、复制状态或改 request ID。
- DAG 对上游失败会跳过后代，但继续执行独立分支；发现环会停止，不采用“把环中节点一起提交”的降级行为。
- `pavo canvas run` 不自动重试。出现超时、断网或响应不明确时，先读取节点的 `task_id` 或按已返回的任务 ID 查询，绝不盲目再次运行，以免重复创建任务。
- 仅当用户已明确要求生成时运行生成节点；如果用户只要求搭建、整理或连接画布，停在图结构完成状态。生成不做积分估算，也不额外暂停。
- 删除项目、节点或连线只在用户明确要求时执行，并传 CLI 要求的 `--yes`。不要通过“重建项目”规避一次失败。
- `pavo canvas run --download` 在保留画布远程 URL 的同时，默认把资源保存到工作区 `pavo_outputs/canvas/TASK_ID/`。立即使用成功项的绝对 `local_path`，不要把远程 URL 当作本地路径；单项下载失败时读取该项 `download_error`，不要把已成功的生成任务报告为失败。
- `--download` / `--output-dir` 需要等待终态，不能与 `--wait=false` 同时使用。异步任务完成后如需本地文件，使用任务结果 URL 调用 `pavo download-result`。
- 登录失败时按现有 PAVO 手机验证码流程处理；不得输出验证码、Access Token 或预签名上传 URL。
