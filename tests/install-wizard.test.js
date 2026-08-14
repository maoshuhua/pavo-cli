const assert = require("assert");
const {
  defaultInstallPackage,
  installPackage,
  isGitPackageSpec,
} = require("../scripts/install-wizard");
const { DEFAULT_PKG } = require("../scripts/skills");

const version = require("../package.json").version.replace(/-.*$/, "");

delete process.env.PAVO_CLI_INSTALL_PACKAGE;
assert.strictEqual(defaultInstallPackage(), `${DEFAULT_PKG}@${version}`);
assert.strictEqual(installPackage(), `${DEFAULT_PKG}@${version}`);
assert.strictEqual(
  installPackage("github:maoshuhua/pavo-cli#v0.1.3"),
  "github:maoshuhua/pavo-cli#v0.1.3",
);

process.env.PAVO_CLI_INSTALL_PACKAGE = `${DEFAULT_PKG}@0.2.0`;
assert.strictEqual(installPackage(), `${DEFAULT_PKG}@0.2.0`);

for (const pkg of [
  "github:maoshuhua/pavo-cli#v0.1.3",
  "git+https://github.com/maoshuhua/pavo-cli.git#v0.1.3",
  "https://github.com/maoshuhua/pavo-cli.git#v0.1.3",
]) {
  assert.strictEqual(isGitPackageSpec(pkg), true, `expected Git package spec: ${pkg}`);
}
assert.strictEqual(isGitPackageSpec(`${DEFAULT_PKG}@0.1.3`), false);
