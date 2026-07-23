const assert = require("assert");
const { defaultInstallPackage, installPackage } = require("../scripts/install-wizard");
const { DEFAULT_PKG } = require("../scripts/skills");

const version = require("../package.json").version.replace(/-.*$/, "");

delete process.env.PAVO_CLI_INSTALL_PACKAGE;
assert.strictEqual(defaultInstallPackage(), `${DEFAULT_PKG}@${version}`);
assert.strictEqual(installPackage(), `${DEFAULT_PKG}@${version}`);

process.env.PAVO_CLI_INSTALL_PACKAGE = `${DEFAULT_PKG}@0.2.0`;
assert.strictEqual(installPackage(), `${DEFAULT_PKG}@0.2.0`);
