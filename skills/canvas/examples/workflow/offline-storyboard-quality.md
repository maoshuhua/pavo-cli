# 工作流：离线起草并检查高质量 Storyboard

**用户请求**： “先给我一份 6 镜产品广告分镜文件，检查提示词质量，但暂时不要写入画布或生成。”

**覆盖**：创作 profile、可编辑模板、Schema/连续性 lint、图片与视频提示词编译预览、零后端副作用。

**前置条件**：只需要本地工作目录；该流程不要求登录或绑定画布。template 是待编辑骨架，不是可直接生成的完成稿。

```bash
# 1) 选择题材约束并生成可编辑 JSON
pavo canvas storyboard profile list
pavo canvas storyboard profile show commercial
pavo canvas storyboard template \
  --profile commercial \
  --shots 6 \
  --output "ABSOLUTE_STORYBOARD_JSON_PATH"

# 2) 编辑文件：替换所有“请填写”；产品写入 subjects[]，每镜以 subject_ids 引用；再补场景、动作和负面约束

# 3) 严格检查；不接触 PAVO 后端
pavo canvas storyboard lint "ABSOLUTE_STORYBOARD_JSON_PATH" --strict

# 4) 使用与 build 相同的编译器预览最终 prompt
pavo canvas storyboard compile \
  "ABSOLUTE_STORYBOARD_JSON_PATH" \
  --kind all \
  --strict \
  --output "ABSOLUTE_COMPILED_PROMPTS_JSON_PATH"
```

**输出与验收**：template 文件是 `pavo.storyboard/v1`；编辑后 lint 返回 `valid=true`、`quality_ready=true`、`errors=0`、`warnings=0`；未绑定真实参考资产时允许有明确 `advisories`。compile 的 shot 数一致，每镜图片提示词包含主体/场景/构图/动作/光线/风格/负面约束，视频提示词另含时长、动作时间线、运镜、声音与结尾；期间没有项目、节点或生成 API 调用。

**失败处理**：Schema error 按 `issues[].path` 修字段；`quality.placeholder` 删除“同上/请填写”等省略表达；描述过短 warning 补具体可观察信息；跨镜 reference advisory 只有在后续绑定画布并创建真实参考资产后才能填写 node key，不得编造。若当前任务明确不接触画布，保留 advisory 并报告其一致性限制。
