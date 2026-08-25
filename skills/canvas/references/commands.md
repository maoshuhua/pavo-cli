# 命令参考

所有命令输出单行 JSON 到 stdout。`PROJECT_ID`、`PROJECT_UUID`、`CANVAS_UUID`、`NODE_KEY`、`TASK_ID` 以命令实际输出为准。项目、绑定、单节点运行和 DAG 输出中的 `canvas_url` 可直接在浏览器打开；URL 路径使用数值 `project_id`，查询参数使用两个 UUID。

## 项目与工作区绑定

```bash
pavo canvas project list
pavo canvas project create --title "TITLE" --cover-url "OPTIONAL_URL" --use
pavo canvas use --project "PROJECT_UUID" --canvas "OPTIONAL_CANVAS_UUID"
pavo canvas status
pavo canvas project show
pavo canvas project update --title "NEW_TITLE"
pavo canvas project duplicate --use
pavo canvas unuse
```

`project create` / `duplicate` 只有传 `--use` 才改写当前工作区绑定。`project delete --yes` 不可恢复，只在用户明确要求删除整个项目时使用。

全部画布子命令都接受继承参数 `--project` 与 `--canvas`，可临时覆盖绑定。

## 节点

```bash
pavo canvas node list
pavo canvas node get "NODE_KEY_OR_EXACT_TITLE"

pavo canvas node create \
  --type image \
  --name "主视觉" \
  --prompt "PROMPT" \
  --model "MODEL_CODE" \
  --x 100 --y 80

pavo canvas node update "NODE" --prompt "NEW_PROMPT" --model "MODEL_CODE"
pavo canvas node update "NODE" --data '{"params":{"count":1}}'
pavo canvas node update "NODE" --unset generation_error_code
pavo canvas node delete "NODE" --yes
```

支持的 `--type`：`text`、`image`、`video`、`audio`、`upload`、`directorNode`、`videoComposition`、`group`。省略坐标时，新节点放到当前最右节点右侧；省略名称时使用与前端一致的自增标题。

对图片、视频或音频节点传 `--model` 时，CLI 会重新查询对应 scene，拒绝不存在、离线或当前账号不可用的模型，并按实时 constraints 写入 `modeType`、默认比例/分辨率、图片数量或视频时长。用户在 `--data.params` 中提供且仍受模型支持的设置会保留；不支持的组合回落到实时配置的首个可用值。

`node update --data` 默认只合并 data 顶层字段，并保留后端返回的未知字段。只有确实需要替换整个 data 时才加 `--replace-data`；CLI 仍会保留 `node_key`。嵌套对象不是递归合并，例如传入 `{"params":...}` 会替换完整 `params`，因此先读取节点并带齐需要保留的 params。

## 上传与连线

```bash
pavo canvas upload --file "ABSOLUTE_MEDIA_PATH" --name "参考素材"
pavo canvas edge add --source "SOURCE_NODE" --target "TARGET_NODE"
pavo canvas edge list
pavo canvas edge delete "CONNECTION_ID" --yes
```

`canvas upload` 自动按 MIME 类型使用 `ugc_image`、`ugc_video` 或 `ugc_audio`，仅返回公共 URL，不输出预签名 URL。它创建不可执行的 `upload` 节点；后续把该节点连接到可执行节点。

连线方向为素材/上游节点 → 生成/下游节点。CLI 只阻止自连、完全重复连线和重复 ID；模型输入数量与媒体类型限制以实时模型配置和服务端校验为准。

## 分组与原子批量搭图

```bash
pavo canvas group create "NODE_A" "NODE_B" --name "镜头组"
pavo canvas group ungroup "GROUP" --yes
pavo canvas apply --stdin --dry-run < workflow.ndjson
pavo canvas apply --stdin < workflow.ndjson
```

`group create` 会拆平被选中的已有 group，按前端相同的 32px 边距创建新 group，并把成员绝对坐标转换为组内相对坐标。`group ungroup` 恢复绝对坐标。

`apply --stdin` 一行读取一个 JSON object，全部解析、结构校验成功后才发出一次原子 batch；`--dry-run` 只输出待提交 batch。流中含 `node.delete`、`edge.delete` 或 `group.ungroup` 时必须增加 `--yes`。完整操作格式见 [automation.md](automation.md)。

## 模型、工具和生成任务

```bash
pavo canvas model list --scene canvas_image
pavo canvas model list --scene canvas_video
pavo canvas model list --scene canvas_audio
pavo canvas tool-specs

pavo canvas run "NODE" --download
pavo canvas run "NODE" --download --output-dir "ABSOLUTE_OUTPUT_DIRECTORY"
pavo canvas run "NODE" --wait=false
pavo canvas task status "TASK_ID"
pavo canvas task wait "TASK_ID" --interval 3s --timeout 30m
pavo canvas task cancel "TASK_ID"
```

`run` 会把已创建的 `task_id`、执行态和最终 URL/文本结果按前端约定回写节点，使网页刷新后仍能恢复和展示。默认轮询至成功或失败，stdout 最终 JSON 的 `task.task_result` 已从后端的 JSON 字符串解码为对象。

传 `--download` 时，成功资源默认保存到当前工作区 `pavo_outputs/canvas/TASK_ID/`；`--output-dir` 可指定其他目录，并会隐式启用下载。每个结果保留远程 `url`，下载成功增加绝对 `local_path`，单项失败增加 `download_error` 并继续处理其他结果。`--download` / `--output-dir` 不能与 `--wait=false` 同时使用。

任务失败也会正常输出 `failed: true`、`error_code` / `error_message`，调用者必须检查，不要仅凭命令退出码判定生成成功。若任务已创建但节点 batch 回写失败，JSON 会保留任务并给出 `sync_error`；按 `task_id` 查询，不要重复运行。

如果节点已有非 `-1` 的 `task_id`，`run` 会拒绝重复提交。`--force` 只适合用户已确认旧任务确实失效的情况，不能作为普通重试方式。

## DAG 计划、执行与恢复

```bash
pavo canvas dag plan --group "GROUP"
pavo canvas dag plan --target "FINAL_NODE"
pavo canvas dag plan --all
pavo canvas dag run --plan "PLAN_ID" --max-parallel 4 --download
pavo canvas dag status "RUN_ID"
pavo canvas dag resume "RUN_ID" --interval 3s --timeout 2h
```

`dag plan` 是只读操作：按 `connection_list` 做拓扑排序和环检测，固定计划内节点的参数摘要和幂等 request ID，并保存 `.pavo/canvas-plans/PLAN_ID.json`。`--target` 会包含可执行祖先；`--group` 会包含组内可执行节点及其祖先；`--all` 选择全部可执行节点。`videoComposition` 按后端能力视为可执行。

`dag run` 必须引用已有 plan。提交前会重读图和节点参数；变化时返回 `replan_required`，不会提交。任务使用同一 `executionBatchId` 和预先固定、从 1 开始的 `batchOrder`；依赖满足的节点最多按 `--max-parallel` 并行。上游失败只跳过其后代，独立分支继续。

运行清单位于 `.pavo/canvas-runs/RUN_ID.json`。进程中断或响应不明确时先 `dag status`，再 `dag resume`；后者对尚未取得 task ID 的节点复用原始幂等 request ID。不要手工改清单或直接改成新 request ID。

## Artifact 历史产物

```bash
pavo canvas artifact list --category all --page 1 --page-size 5
pavo canvas artifact list --category videos --download-dir "ABSOLUTE_DIRECTORY"
pavo canvas artifact save "NODE" --resource-index 0 --name "资产名称"
pavo canvas artifact delete "ARTIFACT_UUID" --yes
pavo canvas artifact delete "UUID_1" "UUID_2" --yes
```

`artifact list` 的 `page_size` 是每页“有产物的日期组”数量，不是产物条数，`pagination.total` 也是日期总数。大整数 ID 以字符串安全输出。指定下载目录时必须是绝对路径，单项下载失败写入 `download_error`，其他项继续。

`artifact save` 的定位参数是节点及 `data.url` 的零基 `resource-index`；不是 artifact UUID 或 URL。后端按节点内容版本做幂等保存。`artifact delete` 只软删历史产物记录，不删除节点当前资源、已保存资产或对象存储文件；批量最多 100 个 UUID。

## 身份验证

先直接执行目标命令。只有 CLI 报告未登录时才进入登录：

```bash
pavo login send-code --country-code "86" --phone-number "PHONE_NUMBER"
pavo login --country-code "86" --phone-number "PHONE_NUMBER"
```

收到用户本次验证码后，非交互调用才增加 `--verification-code "CODE"`。
