# 工作流：查询、下载并保存画布产物

**用户请求**： “下载我最近的画布视频，并把最终视频保存到我的资产。”

**覆盖**：历史产物分页、本地下载、节点资源索引、保存到 My Assets。

**前置条件**：下载目录必须是绝对路径；要保存的最终视频节点及其 `data.url` 已存在。

```bash
# 1) 查询最近的视频日期组并下载当前页全部 URL
pavo canvas artifact list \
  --category videos \
  --page 1 \
  --page-size 5 \
  --download-dir "ABSOLUTE_DOWNLOAD_DIRECTORY"

# 2) 读取最终视频节点，确认 data.url 的零基索引
pavo canvas node get "FINAL_VIDEO_NODE"

# 3) 保存指定资源到“我的资产”
pavo canvas artifact save "FINAL_VIDEO_NODE" \
  --resource-index 0 \
  --name "雨夜广告最终版"
```

**输出与验收**：下载成功项有绝对 `local_path`；单项失败只带 `download_error`，其他成功项仍可用；save 输出表明资源已保存。`resource-index` 对应节点 `data.url`，不是 artifact UUID。

**失败处理**：不要因为一个 `download_error` 把生成任务报告为失败。历史列表删除不属于本请求，不能执行 `artifact delete`。
