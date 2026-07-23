const fs = require("fs");
const path = require("path");
const { run, runSilent } = require("./platform");

const DEFAULT_PKG = "@pavo-dev/cli";

function installSkillsFromRoot(root, opts = {}) {
  const source = path.resolve(root);
  const skillsDir = path.join(source, "skills");
  if (!fs.existsSync(skillsDir)) {
    throw new Error(`skills directory not found: ${skillsDir}`);
  }
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
};
