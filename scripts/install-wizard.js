#!/usr/bin/env node

const fs = require("fs");
const path = require("path");
const { isWindows, run, runSilent } = require("./platform");
const { DEFAULT_PKG, installGlobalPackageSkills } = require("./skills");

const VERSION = require("../package.json").version.replace(/-.*$/, "");

function defaultInstallPackage() {
  return `${DEFAULT_PKG}@${VERSION}`;
}

function installPackage() {
  return process.env.PAVO_CLI_INSTALL_PACKAGE || defaultInstallPackage();
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

function main() {
  const pkg = installPackage();
  const installed = getGloballyInstalledVersion();
  console.log(installed ? `Updating global PAVO CLI (${installed}) via ${pkg}...` : `Installing ${pkg} globally...`);
  run("npm", ["install", "-g", pkg], {
    timeout: 120000,
    env: { ...process.env, PAVO_CLI_SKIP_SKILLS: "1" },
  });
  console.log("Installing PAVO desktop-agent skill...");
  installGlobalPackageSkills(DEFAULT_PKG);

  const bin = whichPavo();
  if (!bin) {
    throw new Error("PAVO CLI was installed, but the pavo command was not found in npm PATH");
  }
  console.log(`PAVO CLI is ready: ${bin}`);
  console.log('Try: pavo conversation create --prompt "生成一张美女图"');
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
  installPackage,
  main,
};
