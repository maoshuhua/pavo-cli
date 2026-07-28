const assert = require("assert");
const fs = require("fs");
const path = require("path");

const repoRoot = path.resolve(__dirname, "..");
const skillPath = path.join(repoRoot, "skills", "pavo-skill", "SKILL.md");
const agentMetadataPath = path.join(repoRoot, "skills", "pavo-skill", "agents", "openai.yaml");
const shortDramaSkillPath = path.join(repoRoot, "skills", "short-drama", "SKILL.md");
const shortDramaAgentMetadataPath = path.join(repoRoot, "skills", "short-drama", "agents", "openai.yaml");
const readmePath = path.join(repoRoot, "README.md");

assert.strictEqual(fs.existsSync(skillPath), true, `missing required file: ${skillPath}`);
assert.strictEqual(
  fs.existsSync(agentMetadataPath),
  true,
  `missing recommended file: ${agentMetadataPath}`,
);
assert.strictEqual(fs.existsSync(readmePath), true, `missing required file: ${readmePath}`);
assert.strictEqual(fs.existsSync(shortDramaSkillPath), true, `missing required file: ${shortDramaSkillPath}`);
assert.strictEqual(
  fs.existsSync(shortDramaAgentMetadataPath),
  true,
  `missing recommended file: ${shortDramaAgentMetadataPath}`,
);
assert.strictEqual(fs.existsSync(path.join(repoRoot, "skills", "pavo")), false, "legacy pavo skill directory remains");

const skill = fs.readFileSync(skillPath, "utf8");
const normalizedSkill = skill.replace(/\r\n/g, "\n");
const agentMetadata = fs.readFileSync(agentMetadataPath, "utf8");
const shortDramaSkill = fs.readFileSync(shortDramaSkillPath, "utf8");
const shortDramaAgentMetadata = fs.readFileSync(shortDramaAgentMetadataPath, "utf8");
const readme = fs.readFileSync(readmePath, "utf8");

assert.match(normalizedSkill, /^---\n[\s\S]*?^name:\s*pavo-skill$/m);
assert.doesNotMatch(normalizedSkill, /^user-invocable:/m);
assert.match(agentMetadata, /^interface:$/m);
assert.match(agentMetadata, /\$pavo-skill/);
assert.match(shortDramaSkill, /^---\n[\s\S]*?^name:\s*short-drama$/m);
assert.match(shortDramaAgentMetadata, /^interface:$/m);
assert.match(shortDramaAgentMetadata, /\$short-drama/);

for (const requiredText of [
  "pavo login",
  "pavo upload",
  "pavo conversation create",
  "pavo stream",
  "pavo download-result",
  "GenerationSuccess",
  "mode",
  "design",
	"--download-dir",
	"local_path",
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
  "agnes-video",
]) {
  assert.ok(shortDramaSkill.includes(requiredText), `short-drama skill missing contract: ${requiredText}`);
}

for (const requiredText of ["skills/pavo-skill/", "skills/short-drama/", "pavo short-drama start"]) {
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
