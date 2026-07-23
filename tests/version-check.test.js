const assert = require("assert");
const path = require("path");
const { defaultCacheFile } = require("../scripts/version-check");

assert.strictEqual(
  defaultCacheFile(),
  path.join(require("os").homedir(), ".pavo", "version-check.json"),
);
