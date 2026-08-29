# 失败恢复：单节点异步任务、断线或状态回写失败

**用户请求**： “刚才画布生成断线了，帮我确认是否完成，不要重复生成。”

**覆盖**：异步 task ID、状态查询、终态等待、`sync_error`、避免重复提交。

**前置条件**：已有 `canvas run --wait=false` 输出、节点 `task_id` 或用户提供的 task ID。

```bash
# 正常异步提交示例：立即返回并记录 stdout.task.task_id
pavo canvas run "NODE_KEY" --wait=false

# 断线/响应不明确后，先查询已有 task，不重新 run
pavo canvas task status "TASK_ID"

# 非终态时等待原任务
pavo canvas task wait "TASK_ID" --interval 3s --timeout 30m

# 同时读取节点，核对它仍引用同一个 task_id；异步 wait 不保证补写节点终态
pavo canvas node get "NODE_KEY"
```

**输出与验收**：`task status/wait` 找到同一个 task ID；终态 JSON 的 `terminal=true`；成功时 `failed=false` 并从权威 `task_result` 继续处理；失败时报告 `error_code` 和 `error_message`。只有同步等待的 `canvas run` 会在同一闭环中回写节点终态，异步恢复不能假定节点已有最终 URL/content。

**失败处理**：

- `run` 输出有 `sync_error`：任务可能已经创建，继续按 task ID 查询，不重复提交。
- 节点已有非 `-1` `task_id`：默认拒绝重复运行是正确保护。
- 只有已确认旧 task 永远不会执行且用户明确要求重新生成时才使用 `--force`；不能把它当普通重试。
- 异步模式不能使用 `--download`。任务完成后如需本地文件，可用任务结果 URL 调用 `pavo download-result`。
