#!/usr/bin/env node

const fs = require("fs");
const os = require("os");
const path = require("path");
const { isWindows, run, runSilent } = require("./platform");
const { DEFAULT_PKG, installGlobalPackageSkills } = require("./skills");

const VERSION = require("../package.json").version.replace(/-.*$/, "");

function defaultInstallPackage() {
  return `${DEFAULT_PKG}@${VERSION}`;
}

function installPackage(override) {
  return override || process.env.PAVO_CLI_INSTALL_PACKAGE || defaultInstallPackage();
}

function isGitPackageSpec(pkg) {
  return /^(?:github:|git(?:\+[^:]+)?:|https?:\/\/github\.com\/)/i.test(pkg);
}

function materializeInstallPackage(pkg) {
  if (!isGitPackageSpec(pkg)) {
    return { cleanup() {}, target: pkg };
  }

  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "pavo-npm-pack-"));
  try {
    console.log(`Preparing npm package from ${pkg}...`);
    const raw = runSilent("npm", ["pack", pkg, "--ignore-scripts", "--json"], {
      cwd: tempDir,
      timeout: 120000,
    }).toString();
    const result = JSON.parse(raw);
    if (!Array.isArray(result) || !result[0] || !result[0].filename) {
      throw new Error("npm pack did not return a package filename");
    }
    const target = path.resolve(tempDir, result[0].filename);
    const expectedPrefix = tempDir.endsWith(path.sep) ? tempDir : tempDir + path.sep;
    if (!target.startsWith(expectedPrefix) || !fs.existsSync(target)) {
      throw new Error(`npm pack output not found: ${target}`);
    }
    return {
      target,
      cleanup() {
        fs.rmSync(tempDir, { force: true, recursive: true });
      },
    };
  } catch (err) {
    fs.rmSync(tempDir, { force: true, recursive: true });
    throw err;
  }
}

function getGloballyInstalledVersion() {
  try {
    const out = runSilent("npm", ["list", "-g", DEFAULT_PKG], { timeout: 15000 });
    const match = out.toString().match(/@(\d+\.\d+\.\d+[^\s]*)/);
    return match ? match[1] : "unknown";
  } catch (_) {
    return null;
  }
}

function whichPavo() {
  try {
    const prefix = runSilent("npm", ["prefix", "-g"], { timeout: 15000 })
      .toString()
      .trim();
    const bin = isWindows ? path.join(prefix, "pavo.cmd") : path.join(prefix, "bin", "pavo");
    if (fs.existsSync(bin)) return bin;
  } catch (_) {
    // Fall back to PATH lookup.
  }
  try {
    return runSilent(isWindows ? "where" : "which", ["pavo"])
      .toString()
      .split("\n")[0]
      .trim();
  } catch (_) {
    return null;
  }
}

function main(override) {
  const pkg = installPackage(override);
  const installed = getGloballyInstalledVersion();
  const materialized = materializeInstallPackage(pkg);
  try {
    console.log(installed ? `Updating global PAVO CLI (${installed}) via ${pkg}...` : `Installing ${pkg} globally...`);
    run("npm", ["install", "-g", materialized.target], {
      timeout: 120000,
      env: { ...process.env, PAVO_CLI_SKIP_SKILLS: "1" },
    });
    console.log("Installing PAVO skills...");
    installGlobalPackageSkills(DEFAULT_PKG);
  } finally {
    materialized.cleanup();
  }

  const bin = whichPavo();
  if (!bin) {
    throw new Error("PAVO CLI was installed, but the pavo command was not found in npm PATH");
  }
  console.log(`PAVO CLI is ready: ${bin}`);
  console.log('Try: pavo generate image --prompt "生成一张美女图"');
}

if (require.main === module) {
  try {
    main();
  } catch (err) {
    console.error(`Failed to install PAVO CLI: ${err.message || err}`);
    process.exit(1);
  }
}

module.exports = {
  defaultInstallPackage,
  isGitPackageSpec,
  installPackage,
  main,
  materializeInstallPackage,
};
