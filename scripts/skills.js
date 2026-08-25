const fs = require("fs");
const os = require("os");
const path = require("path");
const { run, runSilent } = require("./platform");

const DEFAULT_PKG = "@pavo-dev/cli";
const RETIRED_SKILLS = ["pavo-skill", "pavo-media-generation", "pavo-canvas"];

function isRetiredSkillDirectory(candidate, skillName) {
  try {
    const stat = fs.lstatSync(candidate);
    if (stat.isSymbolicLink()) return true;
    if (!stat.isDirectory()) return false;
    const manifest = fs.readFileSync(path.join(candidate, "SKILL.md"), "utf8");
    return new RegExp(`^name:\\s*${skillName}\\s*$`, "m").test(manifest);
  } catch (_) {
    return false;
  }
}

function removeLegacySkill(homeDir = os.homedir()) {
  const candidates = [];
  try {
    for (const entry of fs.readdirSync(homeDir, { withFileTypes: true })) {
      if (entry.isDirectory() && entry.name.startsWith(".") && entry.name !== ".agents") {
        for (const skillName of RETIRED_SKILLS) {
          candidates.push({ path: path.join(homeDir, entry.name, "skills", skillName), skillName });
        }
      }
    }
  } catch (_) {
    // The universal skill path below is still checked when the home directory cannot be listed.
  }
  for (const skillName of RETIRED_SKILLS) {
    candidates.push({ path: path.join(homeDir, ".agents", "skills", skillName), skillName });
  }

  const removed = [];
  for (const candidate of candidates) {
    if (!isRetiredSkillDirectory(candidate.path, candidate.skillName)) continue;
    fs.rmSync(candidate.path, { recursive: true, force: true });
    removed.push(candidate.path);
  }
  return removed;
}

function installSkillsFromRoot(root, opts = {}) {
  const source = path.resolve(root);
  const skillsDir = path.join(source, "skills");
  if (!fs.existsSync(skillsDir)) {
    throw new Error(`skills directory not found: ${skillsDir}`);
  }
  removeLegacySkill(opts.homeDir);
  const runner = opts.run || run;
  runner("npx", ["-y", "skills", "add", source, "-g", "--all"], {
    timeout: opts.timeout || 120000,
  });
}

function globalPackageRoot(pkg = DEFAULT_PKG) {
  const npmRoot = runSilent("npm", ["root", "-g"], { timeout: 15000 })
    .toString()
    .trim();
  return path.join(npmRoot, ...pkg.split("/"));
}

function installGlobalPackageSkills(pkg = DEFAULT_PKG, opts = {}) {
  installSkillsFromRoot(globalPackageRoot(pkg), opts);
}

module.exports = {
  DEFAULT_PKG,
  globalPackageRoot,
  installGlobalPackageSkills,
  installSkillsFromRoot,
  removeLegacySkill,
};

if (require.main === module && process.argv[2] === "remove-legacy") {
  const removed = removeLegacySkill();
  console.log(`Removed ${removed.length} legacy PAVO skill path(s)`);
}
