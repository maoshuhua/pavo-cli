# 提示词规范与片段 Schema

## 组合式 prompt

Pixa 的 `data.params.prompt` 是 segment 数组，不是一个只能覆盖的字符串。CLI 的 `node create/update --prompt`、shortcut 和 storyboard 都只替换 `text` segment，并保留已有 `skill` 或媒体 segment。

```bash
pavo canvas node create --type image \
  --skill character_setting \
  --prompt "固定角色五官、黑色短发、米色风衣；雨夜车站中景" \
  --model "LIVE_MODEL"

pavo canvas node update "NODE" \
  --prompt-segments '[{"type":"skill","code":"character_setting"},{"type":"text","content":"完整主体与镜头描述"}]'
```

`--skill` 可重复。`--prompt-segments` 用于明确替换整个数组，必须是 JSON array；普通修改优先使用 `--prompt`，避免丢失前端写入的新 segment 字段。

## 单镜图片模板

手写单镜提示词时按以下顺序给出具体内容，不写空泛形容词：

1. 分镜目标：本镜承担的剧情信息。
2. 角色与主体一致性：人物写姓名、固定外观和服装；产品/道具写结构、材质、颜色、比例、标识及使用状态。
3. 场景一致性：空间、时间、天气、固定陈设。
4. 构图与机位：景别、角度、主体位置、前中后景。
5. 动作与表情：可观察的瞬间，不用“很有感觉”。
6. 光线与色彩：主光方向、冷暖、色板。
7. 统一视觉风格：媒介、质感、画幅、连续性规则。
8. 负面约束：水印、文字、肢体错误、身份/服装/背景漂移等。

## 单镜视频模板

视频除上述一致性字段外还必须写清：时长、起始画面、动作时间线、运镜、表演、声音、结束状态/转场，以及闪烁、跳切、身份漂移和镜头抖动等负面约束。不要只把图片提示词加上“动起来”。

多镜连续内容不要逐条即兴写提示词，使用 `storyboard` Schema；前端已有角色/场景/首尾帧能力时使用 `shortcut`。完成后运行：

```bash
pavo canvas validate "NODE" --strict
```

已有 storyboard 文件时，用离线编译查看真正会写入节点的提示词：

```bash
pavo canvas storyboard lint storyboard.json --strict
pavo canvas storyboard compile storyboard.json --kind all --strict
```

`compile` 和 `storyboard build` 共用同一编译器，不要在 compile 之后再由 Agent 任意润色每镜 prompt，否则会重新引入角色、产品/道具、服装、场景、画幅和负面约束漂移。需要修改时改结构化 storyboard 字段并重新 lint/compile/build。
