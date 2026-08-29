# 工作流：选择或新建画布并绑定工作目录

**用户请求**： “把当前目录绑定到我的产品视觉画布；没有的话就新建一个。”

**覆盖**：项目发现、避免误选、新建默认画布、工作区绑定、网页链接。

**前置条件**：当前目录是用户希望保存 `.pavo/canvas.json` 的工作目录。

```bash
# 1) 先看当前目录是否已有有效绑定
pavo canvas status

# 2A) 用户要使用已有项目：列出候选，再用实际 UUID 绑定
pavo canvas project list
pavo canvas use --project "PROJECT_UUID" --canvas "OPTIONAL_CANVAS_UUID"

# 2B) 用户明确要求新建时才执行；--use 同时写入当前目录绑定
pavo canvas project create --title "产品视觉方案" --use

# 3) 再读一次有效状态
pavo canvas status
```

**输出与验收**：

- 最终 JSON 有 `project_uuid`、`canvas_uuid`、`session_id` 和非空 `canvas_url`。
- 当前目录或父目录出现 `.pavo/canvas.json`，其中不包含 Access Token。
- 最终回复返回 `canvas_url`，不要自行拼 URL。

**失败处理**：已有多个语义相近项目时让用户选择，不按列表第一项猜测。绑定失效时重新 `project list` / `use`；不要手工编辑 `.pavo/canvas.json`。
