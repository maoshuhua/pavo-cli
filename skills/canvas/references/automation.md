# 批量搭图与 DAG 执行

## stdin NDJSON

每个非空行必须是一个 JSON object，并包含 `op`。建议先保存为文件并 dry-run：

```bash
pavo canvas apply --stdin --dry-run < workflow.ndjson
pavo canvas apply --stdin < workflow.ndjson
pavo canvas apply --file workflow.ndjson --dry-run
pavo canvas apply --file workflow.ndjson
```

支持的图操作：

```ndjson
{"op":"node.create","as":"prompt","type":"text","name":"文案","prompt":"海边产品广告"}
{"op":"node.create","as":"image","type":"image","name":"主视觉","model":"MODEL_CODE","data":{"params":{"model":"MODEL_CODE","count":1}}}
{"op":"node.update","ref":"$image","data":{"future_field":"kept"}}
{"op":"edge.add","as":"prompt_to_image","source":"$prompt","target":"$image","role":"prompt","media_order":0}
{"op":"group.create","as":"scene","members":["$prompt","$image"],"name":"场景一"}
{"op":"edge.delete","id":"$prompt_to_image"}
{"op":"group.ungroup","ref":"$scene"}
{"op":"node.delete","ref":"$image"}
```

`as` 定义仅在本次流内有效的别名，后续用 `$alias` 引用；CLI 第一次编译时生成的 node key、connection ID 会在一次版本冲突重放中保持稳定。未知字段、前向引用、重复别名、悬空连线与无效 parent 会报告行号和 op，且不提交任何变更。

节点可用字段：

- `node.create`：`as,type,name,prompt,model,media_type,data,x,y,width,height,parent`
- `node.update`：`ref,name,prompt,model,data,replace_data,unset,x,y,width,height,parent`
- `node.delete`：`ref`

连线可用字段除 `source,target,id` 外，还包括 handles、port types、`role`、`media_order`、`connection_type`、`color_key`、`selectable`、`deletable` 与 `style`。Group 可设置 `members,name,mode_code,border,fill,padding`。

NDJSON 只修改画布图。上传文件、模型查询、生成任务与 artifact 删除不能加入流，因为这些操作不能与 `nodes/batch` 构成同一个事务。`--file` 适合可审阅、可复用的工作流，`--stdin` 适合临时管道；两者不能同时传。含删除或解组 op 的实际提交要传 `--yes`。

## DAG 范围与计划

从下列范围中选一个：

```bash
pavo canvas dag plan --target "FINAL_NODE"
pavo canvas dag plan --group "GROUP"
pavo canvas dag plan --all
```

计划以 `connection_list` 为依赖事实，使用稳定拓扑顺序；同一 level 可并行。任何依赖环都会返回完整环路径并停止。读取输出：

- `nodes[].dependencies` 与 `levels`：实际执行拓扑。
- `nodes[].content_hash`：剔除运行态结果后的节点参数摘要。
- `plan_id` / `plan_hash`：当前拓扑和参数对应的计划身份。
- `canvas_url`：可直接打开的 PAVO 网页画布。

用户已经明确要求执行该范围时：

```bash
pavo canvas dag run --plan "PLAN_ID" --max-parallel 4 --download
```

执行前 CLI 会重新计算结构和节点参数；返回 `replan_required` 时重新 plan。运行中，上游失败把后代标成 `skipped`，但不取消已运行任务，独立分支继续。成功、失败或跳过以 `run.nodes[].status` 为准。

## 恢复

```bash
pavo canvas dag status "RUN_ID"
pavo canvas dag resume "RUN_ID" --interval 3s --timeout 2h
```

后端没有 execution batch 聚合查询，因此 CLI 用 `.pavo/canvas-runs/RUN_ID.json` 记录每个节点的 request ID、task ID 与状态。`status` 批量刷新已有 task ID；`resume` 轮询运行中任务，并对提交结果不明确的节点复用原 request ID。不要另行调用单节点 `canvas run`，也不要手工修改清单。
