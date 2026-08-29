# 结构化 Storyboard

Storyboard 是 CLI-only 的 `pavo.storyboard/v1` Schema，持久化在普通 text 节点的 `data.pavo_storyboard` 中，不要求 agnes_core 新增节点类型。网页端仍能显示该文本节点；CLI 负责验证和把每条 shot 编译成现有 image/video 节点。

规范 JSON Schema 可直接输出给其他 agent、编辑器或校验器：

```bash
pavo canvas storyboard schema
```

## 离线起草、质量检查与提示词预览

这些命令不登录、不读取画布、也不调用生成模型：

```bash
pavo canvas storyboard profile list
pavo canvas storyboard profile show commercial
pavo canvas storyboard template --profile commercial --shots 6 --output storyboard.json
pavo canvas storyboard lint storyboard.json --strict
pavo canvas storyboard compile storyboard.json --kind all --strict --output prompts.json
```

profile 只约束分镜脚本的媒介、叙事和连续性规则；它不等于来自前端实时 tool-specs 的 shortcut。`lint` 的 `valid` 只表示 Schema 可执行，`quality_ready` 还要求没有省略词、描述过短等质量 warning。跨镜头缺少参考节点属于 advisory：不阻塞 `--strict`，但高一致性任务应尽量补真实资产。template 有意包含“请填写”骨架，因此编辑完成前 `--strict` 必须失败。

`compile` 使用与 build 相同的图片和视频编译器，输出 `shots[].image_prompt` / `video_prompt`、引用 node key 和总时长。先看 compile 结果再搭图，可以发现自由发挥、角色/产品/道具信息遗漏、图片 prompt 直接套视频等问题。

## 标准闭环

```bash
pavo canvas storyboard create \
  --profile cinematic \
  --title "雨夜重逢" \
  --brief "两位旧友在老车站重逢，写实电影感，人物与场景连续" \
  --shots 8

pavo canvas storyboard generate "STORYBOARD_NODE"
pavo canvas storyboard show "STORYBOARD_NODE"
pavo canvas storyboard validate "STORYBOARD_NODE" --strict

pavo canvas storyboard build "STORYBOARD_NODE" \
  --image-model "LIVE_IMAGE_MODEL" \
  --with-video \
  --video-model "LIVE_VIDEO_MODEL" \
  --dry-run \
  --strict

pavo canvas storyboard build "STORYBOARD_NODE" \
  --image-model "LIVE_IMAGE_MODEL" \
  --with-video \
  --video-model "LIVE_VIDEO_MODEL" \
  --strict
```

`create` 只创建可执行的文本请求节点；`generate` 才调用文本模型并自动 finalize。若用户自己运行了 `canvas run`，随后用 `storyboard finalize NODE`。finalize 会拒绝未知/缺失字段、重复 ID、非法引用、非连续 order、shot 数为空或 1–30 秒以外的时长。

`build` 会稳定按 `storyboard_node + shot.id + kind` 查找已有资产节点：已存在则同步提示词和模型，不存在则创建。图片关键帧使用固定字段顺序；视频使用独立的时长/时间线/运镜/声音/结尾模板。引用自 character/subject/scene 的 `reference_node_keys` 会连到关键帧；关键帧连到同 shot 视频；生成节点归入 Storyboard group。build 不执行节点。相同输入重复 build 不调用 batch，输出 `changed:false`；这表示节点已经最新，不是失败。

build 完成且用户明确要求生成时：

```bash
pavo canvas validate --all --strict
pavo canvas dag plan --group "STORYBOARD_GROUP_KEY"
pavo canvas dag run --plan "PLAN_ID" --download
```

## 导入与导出

```bash
pavo canvas storyboard import storyboard.json
pavo canvas storyboard import - < storyboard.json
pavo canvas storyboard import revised.json --node "STORYBOARD_NODE"
pavo canvas storyboard export "STORYBOARD_NODE" --output storyboard.json
```

导入必须满足：

- `schema_version` 为 `pavo.storyboard/v1`。
- `title`、`style_bible.visual_style` 非空。
- character、subject、scene、shot ID 各自唯一，shot order 从 1 连续递增。
- 每条 shot 有 `plot`、`shot_size`、`action`，时长为 1–30 秒。
- `character_ids` / `subject_ids` / `scene_id` 只能引用已定义条目。人物使用 `characters`；产品、关键道具、车辆等固定非人物主体使用 `subjects`，其中写清结构、材质、颜色、比例、标识和连续性锚点。

`reference_node_keys` 是可选的真实画布节点引用。需要角色、固定主体或场景参考一致性时先 upload 或用 shortcut 生成设定资产，确认 node key 后再写入 Schema。
