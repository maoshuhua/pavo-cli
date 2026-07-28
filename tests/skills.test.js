const assert = require("assert");
const fs = require("fs");
const path = require("path");

const repoRoot = path.resolve(__dirname, "..");
const skillPath = path.join(repoRoot, "skills", "pavo", "SKILL.md");
const agentMetadataPath = path.join(repoRoot, "skills", "pavo", "agents", "openai.yaml");
const readmePath = path.join(repoRoot, "README.md");

assert.strictEqual(fs.existsSync(skillPath), true, `missing required file: ${skillPath}`);
assert.strictEqual(
  fs.existsSync(agentMetadataPath),
  true,
  `missing recommended file: ${agentMetadataPath}`,
);
assert.strictEqual(fs.existsSync(readmePath), true, `missing required file: ${readmePath}`);

const skill = fs.readFileSync(skillPath, "utf8");
const agentMetadata = fs.readFileSync(agentMetadataPath, "utf8");
const readme = fs.readFileSync(readmePath, "utf8");

assert.match(skill, /^---\n[\s\S]*?^name:\s*pavo$/m);
assert.doesNotMatch(skill, /^user-invocable:/m);
assert.match(agentMetadata, /^interface:$/m);
assert.match(agentMetadata, /\$pavo/);

for (const requiredText of [
  "pavo login",
  "pavo upload",
  "pavo conversation create",
  "pavo stream",
  "pavo download-result",
  "GenerationSuccess",
  "mode",
  "design",
]) {
  assert.ok(skill.includes(requiredText), `pavo skill missing contract: ${requiredText}`);
}

for (const requiredText of ["skills/pavo/", "pavo conversation create", "pavo stream"]) {
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
