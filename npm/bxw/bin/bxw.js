#!/usr/bin/env node

const { spawnSync } = require("child_process");
const path = require("path");
const fs = require("fs");

const platform = process.platform;
const arch = process.arch;

if (platform !== "win32") {
  console.error(`bxw is designed natively for Windows (win32). Your platform is ${platform}. For macOS, use @facets-cloud/bx.`);
  process.exit(1);
}

const pkgName = `bxw-${platform}-${arch}`;
let binPath;

// 1. Try resolving binary in platform-specific subpackage
try {
  const pkgDir = path.dirname(require.resolve(`${pkgName}/package.json`));
  const candidate = path.join(pkgDir, "bin", "bxw.exe");
  if (fs.existsSync(candidate)) {
    binPath = candidate;
  }
} catch (_) { }

// 2. Fallback to local root build or sibling subpackage for development
if (!binPath) {
  const localCandidates = [
    path.join(__dirname, "..", "..", pkgName, "bin", "bxw.exe"),
    path.join(__dirname, "..", "..", "bxw.exe"),
    path.join(__dirname, "bxw.exe")
  ];
  for (const c of localCandidates) {
    if (fs.existsSync(c)) {
      binPath = c;
      break;
    }
  }
}

if (!binPath) {
  console.error(`bxw error: could not locate binary for ${pkgName}. Try reinstalling: npm i -g bxw`);
  process.exit(1);
}

const result = spawnSync(binPath, process.argv.slice(2), {
  stdio: "inherit",
  windowsHide: false,
});

if (result.error) {
  console.error("Failed to execute bxw:", result.error);
  process.exit(1);
}

process.exit(result.status ?? 0);
