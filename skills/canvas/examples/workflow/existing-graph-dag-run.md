# 工作流：检查已有画布 → DAG 计划 → 执行

**用户请求**： “这个画布节点已经搭好了，帮我把最终视频及其依赖跑完。”

**覆盖**：只读图检查、target 范围、拓扑计划、内容哈希保护、执行结果检查。

**前置条件**：目标节点已存在且用户明确要求生成；不需要创建新节点。

```bash
# 1) 先检查当前图，不修改节点
pavo canvas status
pavo canvas node list
pavo canvas edge list
pavo canvas validate --all --strict

# 2) 计划最终节点及所有可执行祖先，记录 plan_id/plan_hash
pavo canvas dag plan --target "FINAL_VIDEO_NODE"

# 3) 用户已明确要求生成，执行固定计划
pavo canvas dag run --plan "PLAN_ID" --max-parallel 4 --download

# 4) 需要刷新已保存运行状态时使用返回的 run_id
pavo canvas dag status "RUN_ID"
```

**输出与验收**：计划包含最终节点及其可执行祖先，不包含无关分支；`nodes[].dependencies` 与画布连线一致；执行后检查每个 `run.nodes[].status`，不能只看命令退出码；最终回复包含 `canvas_url`。

**失败处理**：发现依赖环时停止并报告环路径，不删除边猜测修复。计划后参数变化时不要继续旧 plan；按照 `dag-replan-required` 案例重新规划。
