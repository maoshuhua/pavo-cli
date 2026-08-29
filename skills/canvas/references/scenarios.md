# 场景选择

| 用户意图 | 使用能力 | 是否创建节点 | 是否立即生成 | 完整案例 |
|---|---|---:|---:|---|
| 新建一个普通图片/视频节点 | `canvas node create` | 是 | 否 | [image](../examples/node-types/image.md) / [video](../examples/node-types/video.md) |
| 应用角色设定、场景设定等前端预设 | `canvas shortcut apply SKILL_CODE --source ...` | 通常是 | 仅加 `--run` 时 | [character-setting-shortcut](../examples/workflow/character-setting-shortcut.md) |
| 应用首尾帧、图生提示词等引导模板 | `canvas shortcut apply GUIDE_CODE --input ...` | 按实时模板 | 仅加 `--run` 时 | [first-last-frame-guide](../examples/workflow/first-last-frame-guide.md) |
| 把剧情拆成连续镜头 | `canvas storyboard create/generate/build` | create/build 会 | 否 | [storyboard-to-images](../examples/workflow/storyboard-to-images.md) |
| 已有一批无依赖节点要批量改图 | `canvas apply --stdin` | 取决于 NDJSON | 否 | [ndjson-atomic-workflow](../examples/workflow/ndjson-atomic-workflow.md) |
| 已有多个生成节点，按依赖执行 | `canvas dag plan/run` | plan/run 不创建画布节点 | `dag run` 会 | [existing-graph-dag-run](../examples/workflow/existing-graph-dag-run.md) |
| 只执行一个已经存在的节点 | `canvas run NODE` | 否 | 是 | [generation-resume](../examples/failures/generation-resume.md) |

判断顺序：

1. 用户说的是前端已命名工具或 preset：shortcut。
2. 用户说的是“脚本、分镜、连续若干镜头”：storyboard。
3. 用户已经有完整节点图，只要求并行/按依赖生成：DAG。
4. 用户明确给出底层节点/边清单：node/edge 或 NDJSON。

Shortcut/storyboard 是“怎么搭建和规范节点”，DAG 是“已有节点按什么顺序执行”，两者可串联但不互相替代。Storyboard build 会创建 shot 节点，DAG 不会创建节点。

完整案例总索引见 [examples/README.md](../examples/README.md)。
