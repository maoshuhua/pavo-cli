const assert = require("assert");
const fs = require("fs");
const os = require("os");
const path = require("path");

const repoRoot = path.resolve(__dirname, "..");
const removedSkillDir = path.join(repoRoot, "skills", "pavo-skill");
const removedMediaSkillDir = path.join(repoRoot, "skills", "pavo-media-generation");
const removedCanvasSkillDir = path.join(repoRoot, "skills", "pavo-canvas");
const shortDramaSkillPath = path.join(repoRoot, "skills", "short-drama", "SKILL.md");
const shortDramaAgentMetadataPath = path.join(repoRoot, "skills", "short-drama", "agents", "openai.yaml");
const mediaSkillPath = path.join(repoRoot, "skills", "media-generation", "SKILL.md");
const mediaAgentMetadataPath = path.join(repoRoot, "skills", "media-generation", "agents", "openai.yaml");
const canvasSkillPath = path.join(repoRoot, "skills", "canvas", "SKILL.md");
const canvasAgentMetadataPath = path.join(repoRoot, "skills", "canvas", "agents", "openai.yaml");
const canvasCommandsPath = path.join(repoRoot, "skills", "canvas", "references", "commands.md");
const canvasNodeDataPath = path.join(repoRoot, "skills", "canvas", "references", "node-data.md");
const canvasAutomationPath = path.join(repoRoot, "skills", "canvas", "references", "automation.md");
const canvasStoryboardPath = path.join(repoRoot, "skills", "canvas", "references", "storyboard.md");
const canvasExamplesDir = path.join(repoRoot, "skills", "canvas", "examples");
const canvasExamplesIndexPath = path.join(canvasExamplesDir, "README.md");
const readmePath = path.join(repoRoot, "README.md");

assert.strictEqual(fs.existsSync(removedSkillDir), false, "removed pavo-skill directory remains");
assert.strictEqual(fs.existsSync(removedMediaSkillDir), false, "removed pavo-media-generation directory remains");
assert.strictEqual(fs.existsSync(removedCanvasSkillDir), false, "removed pavo-canvas directory remains");
assert.strictEqual(fs.existsSync(path.join(repoRoot, "skills", "pavo")), false, "legacy pavo skill directory remains");
assert.strictEqual(fs.existsSync(shortDramaSkillPath), true, `missing required file: ${shortDramaSkillPath}`);
assert.strictEqual(fs.existsSync(mediaSkillPath), true, `missing required file: ${mediaSkillPath}`);
assert.strictEqual(fs.existsSync(readmePath), true, `missing required file: ${readmePath}`);
assert.strictEqual(fs.existsSync(shortDramaAgentMetadataPath), true, `missing recommended file: ${shortDramaAgentMetadataPath}`);
assert.strictEqual(fs.existsSync(mediaAgentMetadataPath), true, `missing recommended file: ${mediaAgentMetadataPath}`);
assert.strictEqual(fs.existsSync(canvasSkillPath), true, `missing required file: ${canvasSkillPath}`);
assert.strictEqual(fs.existsSync(canvasAgentMetadataPath), true, `missing recommended file: ${canvasAgentMetadataPath}`);
assert.strictEqual(fs.existsSync(canvasCommandsPath), true, `missing required file: ${canvasCommandsPath}`);
assert.strictEqual(fs.existsSync(canvasNodeDataPath), true, `missing required file: ${canvasNodeDataPath}`);
assert.strictEqual(fs.existsSync(canvasAutomationPath), true, `missing required file: ${canvasAutomationPath}`);
assert.strictEqual(fs.existsSync(canvasStoryboardPath), true, `missing required file: ${canvasStoryboardPath}`);
assert.strictEqual(fs.existsSync(canvasExamplesIndexPath), true, `missing required file: ${canvasExamplesIndexPath}`);

const shortDramaSkill = fs.readFileSync(shortDramaSkillPath, "utf8").replace(/\r\n/g, "\n");
const shortDramaAgentMetadata = fs.readFileSync(shortDramaAgentMetadataPath, "utf8");
const mediaSkill = fs.readFileSync(mediaSkillPath, "utf8").replace(/\r\n/g, "\n");
const mediaAgentMetadata = fs.readFileSync(mediaAgentMetadataPath, "utf8");
const canvasSkill = fs.readFileSync(canvasSkillPath, "utf8").replace(/\r\n/g, "\n");
const canvasAgentMetadata = fs.readFileSync(canvasAgentMetadataPath, "utf8");
const canvasCommands = fs.readFileSync(canvasCommandsPath, "utf8");
const canvasNodeData = fs.readFileSync(canvasNodeDataPath, "utf8");
const canvasAutomation = fs.readFileSync(canvasAutomationPath, "utf8");
const canvasStoryboard = fs.readFileSync(canvasStoryboardPath, "utf8");
const canvasExamplesIndex = fs.readFileSync(canvasExamplesIndexPath, "utf8");
const readme = fs.readFileSync(readmePath, "utf8");

assert.match(shortDramaSkill, /^---\n[\s\S]*?^name:\s*short-drama$/m);
assert.match(shortDramaAgentMetadata, /^interface:$/m);
assert.match(shortDramaAgentMetadata, /\$short-drama/);
assert.match(mediaSkill, /^---\n[\s\S]*?^name:\s*media-generation$/m);
assert.match(mediaAgentMetadata, /^interface:$/m);
assert.match(mediaAgentMetadata, /\$media-generation/);
assert.match(canvasSkill, /^---\n[\s\S]*?^name:\s*canvas$/m);
assert.match(canvasAgentMetadata, /^interface:$/m);
assert.match(canvasAgentMetadata, /\$canvas/);
assert.ok(canvasSkill.includes("examples/README.md"), "canvas skill does not route to examples index");

const canvasExampleCases = [
  "workflow/workspace-setup.md",
  "workflow/upload-create-run.md",
  "workflow/character-setting-shortcut.md",
  "workflow/first-last-frame-guide.md",
  "workflow/offline-storyboard-quality.md",
  "workflow/storyboard-to-images.md",
  "workflow/storyboard-to-video-dag.md",
  "workflow/existing-graph-dag-run.md",
  "workflow/ndjson-atomic-workflow.md",
  "workflow/artifact-download.md",
  "node-types/text.md",
  "node-types/image.md",
  "node-types/video.md",
  "node-types/audio.md",
  "node-types/upload.md",
  "node-types/storyboard.md",
  "failures/validation-errors.md",
  "failures/generation-resume.md",
  "failures/dag-replan-required.md",
];
for (const relative of canvasExampleCases) {
  const casePath = path.join(canvasExamplesDir, ...relative.split("/"));
  assert.strictEqual(fs.existsSync(casePath), true, `missing canvas example case: ${relative}`);
  assert.ok(canvasExamplesIndex.includes(`(${relative})`), `canvas examples index does not link case: ${relative}`);
  const content = fs.readFileSync(casePath, "utf8");
  assert.ok(content.includes("**用户请求**"), `canvas example lacks user request: ${relative}`);
  assert.ok(content.includes("**覆盖**"), `canvas example lacks stated coverage: ${relative}`);
  assert.ok(content.includes("**前置条件**"), `canvas example lacks preconditions: ${relative}`);
  assert.ok(content.includes("**输出与验收**"), `canvas example lacks observable acceptance criteria: ${relative}`);
  assert.ok(content.includes("**失败处理**"), `canvas example lacks failure handling: ${relative}`);
  assert.ok(content.includes("pavo canvas"), `canvas example has no executable canvas command: ${relative}`);
  assert.match(content, /```(?:bash|ndjson)/, `canvas example has no fenced command/data block: ${relative}`);
}

function markdownFiles(root) {
  const result = [];
  for (const entry of fs.readdirSync(root, { withFileTypes: true })) {
    const candidate = path.join(root, entry.name);
    if (entry.isDirectory()) result.push(...markdownFiles(candidate));
    else if (entry.isFile() && entry.name.endsWith(".md")) result.push(candidate);
  }
  return result;
}

for (const markdownPath of markdownFiles(path.join(repoRoot, "skills", "canvas"))) {
  const content = fs.readFileSync(markdownPath, "utf8");
  for (const match of content.matchAll(/\[[^\]]+\]\(([^)#]+\.md)(?:#[^)]+)?\)/g)) {
    if (/^[a-z]+:/i.test(match[1])) continue;
    const target = path.resolve(path.dirname(markdownPath), ...match[1].split("/"));
    assert.strictEqual(fs.existsSync(target), true, `broken canvas skill link: ${markdownPath} -> ${match[1]}`);
  }
}

for (const requiredText of [
  "pavo canvas status",
  "pavo canvas project list",
  "pavo canvas project create",
  "pavo canvas node list",
  "pavo canvas edge list",
  "pavo canvas model list",
  "pavo canvas model show",
  "pavo canvas tool-specs",
  "pavo canvas run",
  "pavo canvas dag plan",
  "pavo canvas dag run",
  "pavo canvas artifact list",
  "pavo canvas apply --stdin",
  "pavo canvas apply --file",
  "storyboard lint",
  "storyboard compile",
  ".pavo/canvas.json",
  "canvas_url",
  "不自动重试",
]) {
  assert.ok(canvasSkill.includes(requiredText), `canvas skill missing contract: ${requiredText}`);
}
for (const document of [canvasSkill, mediaSkill, shortDramaSkill, canvasCommands, canvasAutomation, readme]) {
  for (const removedText of ["pavo credits estimate", "pavo canvas power", "request_user_input", "ask_user_question"]) {
    assert.ok(!document.includes(removedText), `document still contains removed pricing/confirmation contract: ${removedText}`);
  }
}
for (const requiredText of ["node create", "node update", "canvas upload", "edge add", "task wait", "--wait=false", "--download", "--output-dir", "local_path", "download_error", "pavo_outputs/canvas/", "--force", "--yes"]) {
  assert.ok(canvasCommands.includes(requiredText), `canvas commands reference missing contract: ${requiredText}`);
}
for (const requiredText of ["storyboard profile list", "storyboard template", "storyboard lint", "storyboard compile", "model show", "apply --file", "changed:false"]) {
  assert.ok(canvasCommands.includes(requiredText), `canvas commands reference missing quality contract: ${requiredText}`);
}
for (const requiredText of ["quality_ready", "advisory", "reference_node_keys", "subjects", "subject_ids", "changed:false", "--strict"]) {
  assert.ok(canvasStoryboard.includes(requiredText), `canvas storyboard reference missing quality contract: ${requiredText}`);
}
for (const requiredText of ["node.create", "edge.add", "group.create", "$alias", "connection_list", "content_hash", "plan_hash", "replan_required", "canvas_url", "request ID", "canvas dag resume"]) {
  assert.ok(canvasAutomation.includes(requiredText), `canvas automation reference missing contract: ${requiredText}`);
}
for (const requiredText of ["node_key", "isExecutable", "params", "source", "target", "--replace-data"]) {
  assert.ok(canvasNodeData.includes(requiredText), `canvas node-data reference missing contract: ${requiredText}`);
}

for (const requiredText of [
  "pavo short-drama start",
  "pavo short-drama reply",
  "pavo short-drama resume",
  "pavo short-drama list",
  "short_drama_final",
  "--download-concurrency 4",
  "local_path",
  "download_error",
  "failed",
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
  "billing_pending",
  "不执行积分估算",
  "遇到计费信息也不暂停",
]) {
  assert.ok(shortDramaSkill.includes(requiredText), `short-drama skill missing contract: ${requiredText}`);
}

for (const requiredText of [
  "pavo models --mode generate_image --online-only",
  "pavo models --mode generate_video --online-only",
  "pavo generate image",
  "pavo generate video",
  "pavo visuals --category images",
  "pavo visuals --category videos",
  "--download-concurrency 4",
  "local_path",
  "download_error",
  "failed",
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
  "不执行积分估算",
  "不额外暂停",
  "参考图中的主体，在沙滩边跳舞",
  "不要仅按图片数量选择",
  "必须在提交生成任务前询问用户",
  "这些图片是作为视频首/尾帧，还是只用于参考人物、风格或内容？",
  "对带 1–2 张图片且用途不明确的任务不要使用 `--video-mode auto`",
]) {
  assert.ok(mediaSkill.includes(requiredText), `media skill missing contract: ${requiredText}`);
}
assert.match(mediaSkill, /`frames_to_video`[^\n]*文生视频[^\n]*首尾帧生视频/);

for (const authDocument of [shortDramaSkill, mediaSkill]) {
  for (const requiredText of ["pavo login send-code", "--country-code", "--phone-number", "--verification-code"]) {
    assert.ok(authDocument.includes(requiredText), `skill missing phone OTP contract: ${requiredText}`);
  }
  assert.doesNotMatch(authDocument, /--email|--password|PAVO_PASSWORD|邮箱和密码/);
}

for (const modelDocument of [shortDramaSkill, mediaSkill, readme]) {
  assert.doesNotMatch(modelDocument, /agnes-video(?!-new)/);
}

for (const requiredText of ["skills/canvas/", "skills/media-generation/", "skills/short-drama/", "pavo canvas project list", "pavo canvas node create", "pavo canvas run", "pavo canvas dag plan", "pavo canvas dag resume", "pavo canvas artifact list", "pavo canvas apply --stdin", "canvas_url", "replan_required", "https://app-test.pavo-ai.cn", "PAVO_APP_BASE_URL", "pavo visuals --category images", "pavo short-drama list", "--download-concurrency 4", "local_path", "download_error", "failed", "pavo short-drama start", "pavo generate image", "pavo_outputs/", "https://api-pavo-test.pavo-ai.cn", "pavo login send-code"]) {
  assert.ok(readme.includes(requiredText), `README missing skill contract: ${requiredText}`);
}
for (const removedText of ["skills/pavo-skill/", "skills/pavo-media-generation/", "pavo conversation create", "pavo stream", "--mode design", '"mode": "design"']) {
  assert.ok(!readme.includes(removedText), `README still documents removed design workflow: ${removedText}`);
}

const { installSkillsFromRoot } = require("../scripts/skills");
const fakeHome = fs.mkdtempSync(path.join(os.tmpdir(), "pavo-skills-test-"));
const fakeUniversalSkill = path.join(fakeHome, ".agents", "skills", "pavo-skill");
const fakeUniversalMediaSkill = path.join(fakeHome, ".agents", "skills", "pavo-media-generation");
const fakeUniversalCanvasSkill = path.join(fakeHome, ".agents", "skills", "pavo-canvas");
const fakeCodexSkills = path.join(fakeHome, ".codex", "skills");
fs.mkdirSync(fakeUniversalSkill, { recursive: true });
fs.mkdirSync(fakeUniversalMediaSkill, { recursive: true });
fs.mkdirSync(fakeUniversalCanvasSkill, { recursive: true });
fs.mkdirSync(fakeCodexSkills, { recursive: true });
fs.writeFileSync(path.join(fakeUniversalSkill, "SKILL.md"), "---\nname: pavo-skill\n---\n");
fs.writeFileSync(path.join(fakeUniversalMediaSkill, "SKILL.md"), "---\nname: pavo-media-generation\n---\n");
fs.writeFileSync(path.join(fakeUniversalCanvasSkill, "SKILL.md"), "---\nname: pavo-canvas\n---\n");
fs.symlinkSync(
  fakeUniversalSkill,
  path.join(fakeCodexSkills, "pavo-skill"),
  process.platform === "win32" ? "junction" : "dir",
);
fs.symlinkSync(
  fakeUniversalMediaSkill,
  path.join(fakeCodexSkills, "pavo-media-generation"),
  process.platform === "win32" ? "junction" : "dir",
);
fs.symlinkSync(
  fakeUniversalCanvasSkill,
  path.join(fakeCodexSkills, "pavo-canvas"),
  process.platform === "win32" ? "junction" : "dir",
);
let invocation;
installSkillsFromRoot(repoRoot, {
  homeDir: fakeHome,
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
assert.strictEqual(fs.existsSync(fakeUniversalSkill), false, "legacy universal skill was not removed");
assert.strictEqual(fs.existsSync(fakeUniversalMediaSkill), false, "retired media skill was not removed");
assert.strictEqual(fs.existsSync(fakeUniversalCanvasSkill), false, "retired canvas skill was not removed");
assert.strictEqual(fs.existsSync(path.join(fakeCodexSkills, "pavo-skill")), false, "legacy agent skill link was not removed");
assert.strictEqual(fs.existsSync(path.join(fakeCodexSkills, "pavo-media-generation")), false, "retired media skill link was not removed");
assert.strictEqual(fs.existsSync(path.join(fakeCodexSkills, "pavo-canvas")), false, "retired canvas skill link was not removed");
fs.rmSync(fakeHome, { recursive: true, force: true });
