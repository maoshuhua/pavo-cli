# 失败恢复：DAG 返回 `replan_required`

**用户请求**： “DAG 执行提示计划过期，继续跑最新画布。”

**覆盖**：计划内容哈希、图/参数漂移、重新规划、旧 run 与新 plan 的边界。

**前置条件**：已有旧 `plan_id`；用户仍明确要求执行同一 target/group 范围。

```bash
# 1) 先读取最新图和校验结果，确认变化不是异常修改
pavo canvas node list
pavo canvas edge list
pavo canvas validate --all --strict

# 2) 使用原来的范围重新 plan；不要继续旧 plan_id
pavo canvas dag plan --target "ORIGINAL_FINAL_NODE"
# 或：pavo canvas dag plan --group "ORIGINAL_GROUP"

# 3) 核对新 levels/dependencies/content_hash，记录新的 plan_id/plan_hash

# 4) 用户仍要求生成，执行新计划
pavo canvas dag run --plan "NEW_PLAN_ID" --max-parallel 4 --download
```

**输出与验收**：新 `plan_hash` 对应当前图和参数；run 接受新 plan，不再返回 `replan_required`；结果按新计划的 `run.nodes[].status` 检查。

**失败处理**：

- `replan_required` 发生在任务提交前，不应使用 `dag resume OLD_PLAN_ID`；resume 接受的是已创建运行的 `RUN_ID`。
- 如果已经有 `run_id` 且只是进程中断，使用 `dag status RUN_ID` / `dag resume RUN_ID`，不要重新 plan。
- 重新 plan 仍检测到环时停止并报告环路径，不自动删除用户连线。
