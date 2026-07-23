#!/usr/bin/env node

const crypto = require("crypto");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { isWindows, run } = require("./platform");
const { installSkillsFromRoot } = require("./skills");

const VERSION = require("../package.json").version.replace(/-.*$/, "");
const REPO = "maoshuhua/pavo-cli";
const NAME = "pavo";
const ROOT = path.join(__dirname, "..");
const BIN_DIR = path.join(ROOT, "bin");

const PLATFORM_MAP = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};

const ARCH_MAP = {
  x64: "amd64",
  arm64: "arm64",
};

const platform = PLATFORM_MAP[process.platform];
const arch = ARCH_MAP[process.arch];
const archiveExt = isWindows ? ".zip" : ".tar.gz";
const archiveName = `${NAME}-${VERSION}-${platform}-${arch}${archiveExt}`;
const releaseURL = `https://github.com/${REPO}/releases/download/v${VERSION}/${archiveName}`;
const dest = path.join(BIN_DIR, NAME + (isWindows ? ".exe" : ""));

function download(url, destPath) {
  const args = [
    "--fail",
    "--location",
    "--silent",
    "--show-error",
    "--connect-timeout",
    "10",
    "--max-time",
    "120",
    "--max-redirs",
    "3",
    "--output",
    destPath,
  ];
  if (isWindows) args.unshift("--ssl-revoke-best-effort");
  args.push(url);
  run("curl", args);
}

function expectedChecksum(name) {
  const checksumsPath = path.join(ROOT, "checksums.txt");
  if (!fs.existsSync(checksumsPath)) {
    console.warn("[WARN] checksums.txt not found, skipping checksum verification");
    return null;
  }
  for (const line of fs.readFileSync(checksumsPath, "utf8").split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const idx = trimmed.indexOf("  ");
    if (idx === -1) continue;
    if (trimmed.slice(idx + 2) === name) return trimmed.slice(0, idx);
  }
  throw new Error(`Checksum entry not found for ${name}`);
}

function verifyChecksum(filePath, expectedHash) {
  if (!expectedHash) return;
  const actual = crypto.createHash("sha256").update(fs.readFileSync(filePath)).digest("hex");
  if (actual.toLowerCase() !== expectedHash.toLowerCase()) {
    throw new Error(
      `Checksum mismatch for ${path.basename(filePath)}: expected ${expectedHash}, got ${actual}`,
    );
  }
}

function extractArchive(archivePath, destDir) {
  if (isWindows) {
    const env = {
      ...process.env,
      PAVO_CLI_ARCHIVE: archivePath,
      PAVO_CLI_DEST: destDir,
    };
    run(
      "powershell.exe",
      [
        "-NoProfile",
        "-ExecutionPolicy",
        "Bypass",
        "-Command",
        "$ErrorActionPreference='Stop';Expand-Archive -LiteralPath $env:PAVO_CLI_ARCHIVE -DestinationPath $env:PAVO_CLI_DEST -Force",
      ],
      { env },
    );
    return;
  }
  run("tar", ["-xzf", archivePath, "-C", destDir]);
}

function install() {
  if (!platform || !arch) {
    throw new Error(`Unsupported platform: ${process.platform}-${process.arch}`);
  }
  fs.mkdirSync(BIN_DIR, { recursive: true });
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "pavo-cli-"));
  const archivePath = path.join(tmpDir, archiveName);
  try {
    download(releaseURL, archivePath);
    verifyChecksum(archivePath, expectedChecksum(archiveName));
    extractArchive(archivePath, tmpDir);
    const extracted = path.join(tmpDir, NAME + (isWindows ? ".exe" : ""));
    fs.copyFileSync(extracted, dest);
    fs.chmodSync(dest, 0o755);
    if (process.env.PAVO_CLI_SKIP_SKILLS !== "1") {
      installSkillsFromRoot(ROOT);
    }
    console.log(`${NAME} v${VERSION} installed successfully`);
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

if (require.main === module) {
  const isNpxPostinstall =
    process.env.npm_command === "exec" && !process.env.PAVO_CLI_RUN;
  if (isNpxPostinstall) process.exit(0);
  try {
    install();
  } catch (err) {
    console.error(`Failed to install ${NAME}: ${err.message || err}`);
    console.error(
      "\nTry:\n" +
        "  npm install -g @pavo-dev/cli\n" +
        `  node "${path.join(__dirname, "install.js")}"\n`,
    );
    process.exit(1);
  }
}

module.exports = {
  archiveName,
  expectedChecksum,
  install,
  verifyChecksum,
};
