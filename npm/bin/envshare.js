#!/usr/bin/env node

// This file is intentionally tiny. It does not lock or unlock anything
// itself, it just passes your words straight through to the real program
// that scripts/install.js downloaded when this package was installed.

const path = require("path");
const fs = require("fs");
const { spawnSync } = require("child_process");

const ending = process.platform === "win32" ? ".exe" : "";
const programPath = path.join(__dirname, "..", "bin", "envshare-program" + ending);

if (!fs.existsSync(programPath)) {
  console.error("envshare: the program has not been downloaded yet.");
  console.error("envshare: try running npm install again, or build it yourself, see the readme.");
  process.exit(1);
}

const result = spawnSync(programPath, process.argv.slice(2), { stdio: "inherit" });
process.exit(result.status === null ? 1 : result.status);

