# 节点 data 契约

CLI 按 `pavo-app-front` 当前 batch 格式写入完整节点：`nodeKey`、`type`、`name`、字符串坐标与尺寸、`parentKey`，以及序列化成字符串的 `data` JSON。后端返回的未知 data 字段会在普通更新中保留。

## 通用字段

新节点至少包含：

```json
{
  "node_key": "i-XXXXXXXXXXXX",
  "title": "图片节点1",
  "isExecutable": true,
  "progress": -1,
  "task_id": "-1"
}
```

`text`、`image`、`video`、`audio`、`videoComposition` 可执行；`upload`、`directorNode`、`group` 默认不可执行。旧前端记录可能把 `videoComposition` 标成不可执行，DAG 与后端执行能力以可执行处理。不要手工指定或修改 `node_key`。

上传节点另有 `mediaType: "image" | "video" | "audio"` 与 `url: ["PUBLIC_URL"]`。使用 `pavo canvas upload` 生成这些字段，不要自行走预签名接口。

## Prompt 与 params

`--prompt` 写入标准片段数组；更新时只替换 `type=text` 的片段，并保留 skill/media 片段：

```json
{
  "params": {
    "prompt": [
      {"type": "text", "content": "一只猫在窗边晒太阳"}
    ]
  }
}
```

文本节点还会同步 `content`，并把 `mode` 设为 `authoring`。模型用 `params.model`：

```json
{
  "params": {
    "model": "LIVE_MODEL_CODE",
    "prompt": [{"type": "text", "content": "PROMPT"}],
    "count": 1,
    "settings": {}
  }
}
```

通过 `--data` 设置模型专属字段前，先读取 `canvas model list` 的对应 scene。传 `--model` 时 CLI 会按实时 constraints 补齐或修正 `modeType`、`settings`、图片 `count` 与视频 `duration`；不要凭其他生成命令的参数推断画布模型设置。

Prompt 片段还可表达画布内引用或工具：

```json
{"type":"image","url":"PUBLIC_URL","source_index":0}
{"type":"video","url":"PUBLIC_URL","source_index":0}
{"type":"audio","url":"PUBLIC_URL","source_index":0}
{"type":"skill","code":"TOOL_CODE"}
```

优先通过画布连线表达节点依赖。只有实时 tool specs 或现有节点明确使用片段时，才手工写 `skill` / 媒体片段。

`node create/update --skill CODE` 会去重并把 skill segment 放在 text 前；`--prompt-segments JSON_ARRAY` 明确替换整个数组。前端 preset 优先用 `canvas shortcut apply`，不要直接猜 skill code。

结构化分镜存储在 text 节点的 `data.pavo_storyboard`，Schema 版本为 `pavo.storyboard/v1`。人物在 `characters[]`，产品/道具/车辆等固定非人物主体在 `subjects[]`，镜头分别用 `character_ids` / `subject_ids` 引用；build 产物使用 `data.pavo_storyboard_asset` 记录所属 storyboard node、shot ID 和 image/video kind，group 使用 `data.pavo_storyboard_group`。这些字段由 `canvas storyboard` 管理，不手工拼接。

## 连线派生字段

创建 A → B 连线时，CLI 与前端一样同步：

```json
// A.data
{"target":["B_NODE_KEY"]}

// B.data
{"source":["A_NODE_KEY"]}
```

删除连线或节点时同步移除这些引用。不要仅用 `node update` 手工改 `source` / `target` 来冒充连线；实际连接以 `connection_list` 为准。

连线 batch 还会完整保留 `source_handle`、`target_handle`、端口类型、`role`、`media_order`、`connection_type`、`color_key`、`selectable`、`deletable` 与 `style_json`。DAG 依赖只以 `connection_list` 的 source/target 为准，不能用 data 中可能过期的 `source` 推断拓扑。

## Group 父子关系

Group 归属的事实来源是子节点 batch 的 `parentKey` 与 `data.parent_key`，两者必须一致。组内节点 position 是相对坐标；顶层节点 position 是画布绝对坐标。不要只改 `parent_key`，使用 `canvas group` 或 NDJSON 的 `group.create` / `group.ungroup`，由 CLI 同步坐标和两个父字段。

## 高级替换

`node update --data` 是顶层 merge，不递归 merge 嵌套对象。更新 `params` 时先执行 `node get`，在本地构造完整的新 params 再提交。

`--replace-data` 会丢弃未显式带入的业务字段，可能使前端无法正确展示或运行节点。它只用于修复已知损坏节点，并应保留至少 `title`、`isExecutable`、`progress`、`task_id` 及该节点类型要求的 params；`node_key` 由 CLI 强制保留。
