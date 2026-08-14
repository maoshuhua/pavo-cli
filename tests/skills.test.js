const assert = require("assert");
const fs = require("fs");
const path = require("path");

const repoRoot = path.resolve(__dirname, "..");
const skillPath = path.join(repoRoot, "skills", "pavo-skill", "SKILL.md");
const agentMetadataPath = path.join(repoRoot, "skills", "pavo-skill", "agents", "openai.yaml");
const shortDramaSkillPath = path.join(repoRoot, "skills", "short-drama", "SKILL.md");
const shortDramaAgentMetadataPath = path.join(repoRoot, "skills", "short-drama", "agents", "openai.yaml");
const mediaSkillPath = path.join(repoRoot, "skills", "pavo-media-generation", "SKILL.md");
const mediaAgentMetadataPath = path.join(repoRoot, "skills", "pavo-media-generation", "agents", "openai.yaml");
const readmePath = path.join(repoRoot, "README.md");

assert.strictEqual(fs.existsSync(skillPath), true, `missing required file: ${skillPath}`);
assert.strictEqual(
  fs.existsSync(agentMetadataPath),
  true,
  `missing recommended file: ${agentMetadataPath}`,
);
assert.strictEqual(fs.existsSync(readmePath), true, `missing required file: ${readmePath}`);
assert.strictEqual(fs.existsSync(shortDramaSkillPath), true, `missing required file: ${shortDramaSkillPath}`);
assert.strictEqual(fs.existsSync(mediaSkillPath), true, `missing required file: ${mediaSkillPath}`);
assert.strictEqual(
  fs.existsSync(shortDramaAgentMetadataPath),
  true,
  `missing recommended file: ${shortDramaAgentMetadataPath}`,
);
assert.strictEqual(
  fs.existsSync(mediaAgentMetadataPath),
  true,
  `missing recommended file: ${mediaAgentMetadataPath}`,
);
assert.strictEqual(fs.existsSync(path.join(repoRoot, "skills", "pavo")), false, "legacy pavo skill directory remains");

const skill = fs.readFileSync(skillPath, "utf8");
const normalizedSkill = skill.replace(/\r\n/g, "\n");
const agentMetadata = fs.readFileSync(agentMetadataPath, "utf8");
const shortDramaSkill = fs.readFileSync(shortDramaSkillPath, "utf8").replace(/\r\n/g, "\n");
const shortDramaAgentMetadata = fs.readFileSync(shortDramaAgentMetadataPath, "utf8");
const mediaSkill = fs.readFileSync(mediaSkillPath, "utf8").replace(/\r\n/g, "\n");
const mediaAgentMetadata = fs.readFileSync(mediaAgentMetadataPath, "utf8");
const readme = fs.readFileSync(readmePath, "utf8");

assert.match(normalizedSkill, /^---\n[\s\S]*?^name:\s*pavo-skill$/m);
assert.doesNotMatch(normalizedSkill, /^user-invocable:/m);
assert.match(agentMetadata, /^interface:$/m);
assert.match(agentMetadata, /\$pavo-skill/);
assert.match(shortDramaSkill, /^---\n[\s\S]*?^name:\s*short-drama$/m);
assert.match(shortDramaAgentMetadata, /^interface:$/m);
assert.match(shortDramaAgentMetadata, /\$short-drama/);
assert.match(mediaSkill, /^---\n[\s\S]*?^name:\s*pavo-media-generation$/m);
assert.match(mediaAgentMetadata, /^interface:$/m);
assert.match(mediaAgentMetadata, /\$pavo-media-generation/);

for (const requiredText of [
  "pavo login",
  "pavo upload",
  "pavo conversation create",
  "pavo stream",
  "pavo download-result",
  "GenerationSuccess",
  "mode",
  "design",
	"$pavo-media-generation",
	"--download-dir",
	"local_path",
	"pavo_outputs/",
]) {
  assert.ok(skill.includes(requiredText), `pavo skill missing contract: ${requiredText}`);
}

for (const requiredText of [
  "pavo short-drama start",
  "pavo short-drama reply",
  "pavo short-drama resume",
  "conversation_id",
  "short_drama",
  "agnes-image",
  "agnes-video-new",
  "--live-assets",
  "asset_ready",
  "complete",
  "pavo_outputs/",
  "pavo models --mode short_drama --type image --online-only",
  "pavo models --mode short_drama --type video --online-only",
  'tags[].code == "free"',
]) {
  assert.ok(shortDramaSkill.includes(requiredText), `short-drama skill missing contract: ${requiredText}`);
}

for (const requiredText of [
  "pavo models --mode generate_image --online-only",
  "pavo models --mode generate_video --online-only",
  "pavo generate image",
  "pavo generate video",
  "creative_prompt_json",
  "agnes-image",
  "agnes-video-new",
  "--image",
  "--video",
  "--audio",
  "--video-mode",
  "--live-assets",
  "asset_ready",
  "pavo_outputs/",
  'tags[].code == "free"',
  "参考图中的主体，在沙滩边跳舞",
  "不要仅按图片数量选择",
  "必须在提交生成任务前询问用户",
  "这些图片是作为视频首/尾帧，还是只用于参考人物、风格或内容？",
  "对带 1–2 张图片且用途不明确的任务不要使用 `--video-mode auto`",
]) {
  assert.ok(mediaSkill.includes(requiredText), `media skill missing contract: ${requiredText}`);
}
assert.match(mediaSkill, /`frames_to_video`[^\n]*文生视频[^\n]*首尾帧生视频/);

for (const authDocument of [skill, shortDramaSkill, mediaSkill]) {
  for (const requiredText of ["pavo login send-code", "--country-code", "--phone-number", "--verification-code"]) {
    assert.ok(authDocument.includes(requiredText), `skill missing phone OTP contract: ${requiredText}`);
  }
  assert.doesNotMatch(authDocument, /--email|--password|PAVO_PASSWORD|邮箱和密码/);
}

for (const modelDocument of [shortDramaSkill, mediaSkill, readme]) {
  assert.doesNotMatch(modelDocument, /agnes-video(?!-new)/);
}

for (const requiredText of ["skills/pavo-skill/", "skills/pavo-media-generation/", "skills/short-drama/", "pavo short-drama start", "pavo generate image", "pavo_outputs/", "https://api.pavo-ai.cn", "pavo login send-code"]) {
  assert.ok(readme.includes(requiredText), `README missing skill contract: ${requiredText}`);
}

const { installSkillsFromRoot } = require("../scripts/skills");
let invocation;
installSkillsFromRoot(repoRoot, {
  timeout: 3210,
  run(command, args, options) {
    invocation = { command, args, options };
  },
});
assert.deepStrictEqual(invocation, {
  command: "npx",
  args: ["-y", "skills", "add", repoRoot, "-g", "--all"],
  options: { timeout: 3210 },
});
