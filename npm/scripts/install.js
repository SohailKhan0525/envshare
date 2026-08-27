// This runs once, automatically, right after someone types the words
// npm install envshare cli (or npm install with a dash, as npm itself
// requires). It does not lock or unlock anything itself, it only fetches
// the correct ready made program for the current computer, from the
// project's GitHub release page, so the person never needs to install the
// Go programming language themselves.

const fs = require("fs");
const path = require("path");
const https = require("https");

// Change these two values if you publish this under your own GitHub
// project, so the install step points at your own releases.
const githubOwner = "SohailKhan0525";
const githubRepo = "envshare";

function currentSystem() {
  const map = { win32: "windows", darwin: "darwin", linux: "linux", android: "linux" };
  const system = map[process.platform];
  if (!system) {
    throw new Error("this computer's system (" + process.platform + ") is not supported yet");
  }
  return system;
}

function currentChip() {
  const map = { x64: "amd64", arm64: "arm64" };
  const chip = map[process.arch];
  if (!chip) {
    throw new Error("this computer's chip type (" + process.arch + ") is not supported yet");
  }
  return chip;
}

function downloadFile(url, destinationPath, redirectsLeft) {
  return new Promise((resolve, reject) => {
    if (redirectsLeft === undefined) redirectsLeft = 5;
    const request = https.get(url, (response) => {
      if (
        response.statusCode >= 300 &&
        response.statusCode < 400 &&
        response.headers.location &&
        redirectsLeft > 0
      ) {
        response.resume();
        downloadFile(response.headers.location, destinationPath, redirectsLeft - 1)
          .then(resolve, reject);
        return;
      }
      if (response.statusCode !== 200) {
        reject(new Error("could not download the program, the server said status " + response.statusCode));
        return;
      }
      const fileStream = fs.createWriteStream(destinationPath);
      response.pipe(fileStream);
      fileStream.on("finish", () => fileStream.close(resolve));
      fileStream.on("error", reject);
    });
    request.on("error", reject);
  });
}

async function main() {
  const system = currentSystem();
  const chip = currentChip();
  const ending = system === "windows" ? ".exe" : "";
  const fileName = "envshare." + system + "." + chip + ending;

  const releaseUrl =
    "https://github.com/" +
    githubOwner +
    "/" +
    githubRepo +
    "/releases/latest/download/" +
    fileName;

  const binDir = path.join(__dirname, "..", "bin");
  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }
  const destination = path.join(binDir, "envshare-program" + ending);

  console.log("envshare: fetching the program built for your computer...");
  try {
    await downloadFile(releaseUrl, destination);
  } catch (error) {
    console.error("envshare: could not automatically download the program.");
    console.error("envshare: you can build it yourself instead, see the project's readme.");
    console.error("envshare: the underlying error was: " + error.message);
    process.exitCode = 0; // do not fail the whole install, just warn
    return;
  }

  if (system !== "windows") {
    fs.chmodSync(destination, 0o755);
  }
  console.log("envshare: ready. Type envshare followed by a command to get started.");
}

main();

