# Shortcut：实时前端预设

Shortcut 把 Pixa `tool-specs` 中的 `guide`、`skill`、`mode` 归一成可发现、可校验、可原子应用的命令。code 和模板内容以实时接口为准，不在 Skill 中维护静态清单。

## 发现与查看

```bash
pavo canvas shortcut list
pavo canvas shortcut list --kind skill --type image
pavo canvas shortcut show character_setting
```

输出的 `required_inputs` 给出 `key`、节点类型和实时模板是否带 example URL。使用 `show` 返回的 key 绑定输入，不猜 `input1`；`input1`、`first`、`last` 只是兼容别名。

## 三种行为

- `skill`：要求 `--source NODE`，创建一个带 `{"type":"skill","code":"..."}` prompt segment 的输出节点，并连接 source → output。
- `guide`：按实时 `extra.node_list` 创建或更新 self/input/output 节点、连线和 group。用可重复的 `--input KEY=NODE` 绑定素材；只有明确需要服务端示例素材时才用 `--use-example-input`。
- `mode`：要求 `--target NODE`，更新目标的 mode 数据，不新建生成任务。

```bash
pavo canvas shortcut apply character_setting \
  --source "角色参考图" \
  --prompt "固定五官、发型、服装和配色" \
  --model "LIVE_IMAGE_MODEL" \
  --dry-run

pavo canvas shortcut apply character_setting \
  --source "角色参考图" \
  --prompt "固定五官、发型、服装和配色" \
  --model "LIVE_IMAGE_MODEL"

pavo canvas shortcut apply guide_first_last_frames \
  --input node1="首帧" \
  --input node2="尾帧" \
  --model "LIVE_VIDEO_MODEL"
```

`--target` 表示复用已有 self 节点；不传时 CLI 创建模板 self 节点。`--prompt` 只替换 text segment，保留 skill 和媒体 segment。`--model` 会经实时模型约束校验；省略时选择当前账号首个在线可用模型。

`shortcut apply` 默认只改图。用户明确要求立即生成时增加 `--run`；要本地文件再增加 `--download` 或 `--output-dir`。返回的 `run_node_key` 是实际可执行目标，`canvas_url` 是网页画布地址。

## 错误处理

- code 不存在：重新 `shortcut list`，不要换成记忆中的旧 code。
- 输入缺失：根据 `required_inputs[].key` 补 `--input`。
- 模型不可用：查询对应 `canvas model list`，选择 `allowed`、在线的 model code。
- 只想看图变更：始终用 `--dry-run`，不要加 `--run`。
