# `upload`：本地素材节点

**用户请求**： “把这张参考图放到当前画布，但先不要生成。”

**覆盖**：MIME 自动识别、公共素材 URL、不可执行 upload 节点。

**前置条件**：用户提供可读取的本地图片、视频或音频绝对路径。

```bash
pavo canvas upload \
  --file "ABSOLUTE_LOCAL_MEDIA_PATH" \
  --name "角色参考素材"

pavo canvas node get "UPLOAD_NODE_KEY"
```

**输出与验收**：上传输出有 `node_key`；节点类型为 upload，`data.mediaType` 与文件 MIME 一致，`data.url` 有公共资源地址，`isExecutable=false`；没有触发任务。

**失败处理**：不要自行请求预签名上传接口，也不要输出预签名 URL。后续引用使用 `edge add` 或 shortcut 的 `--source`/`--input`。
