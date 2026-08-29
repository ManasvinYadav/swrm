#!/usr/bin/env node
'use strict';

// Downloads the prebuilt swrm binary matching this package's version and the
// current platform/arch from GitHub Releases. Pure Node stdlib — no runtime
// dependencies for the npm package.

const fs = require('fs');
const path = require('path');
const https = require('https');

const REPO = 'ManasvinYadav/swrm';
const pkg = require('../package.json');

const PLATFORM_MAP = { darwin: 'darwin', linux: 'linux', win32: 'windows' };
const ARCH_MAP = { x64: 'amd64', arm64: 'arm64' };

function fail(message) {
  console.error(`[swrm install] ${message}`);
  process.exit(1);
}

function resolveTarget() {
  const os = PLATFORM_MAP[process.platform];
  const arch = ARCH_MAP[process.arch];
  if (!os) {
    fail(
      `Unsupported platform "${process.platform}". swrm ships prebuilt binaries for ` +
        'macOS, Linux, and Windows only — try building from source instead.'
    );
  }
  if (!arch) {
    fail(
      `Unsupported architecture "${process.arch}". swrm ships prebuilt binaries for ` +
        'x64 and arm64 only — try building from source instead.'
    );
  }
  const ext = os === 'windows' ? '.exe' : '';
  return { os, arch, ext, assetName: `swrm-${os}-${arch}${ext}` };
}

// Follows 3xx redirects manually (GitHub release assets redirect to
// objects.githubusercontent.com) and streams the response straight to disk.
function download(url, destPath, redirectsLeft) {
  if (redirectsLeft === undefined) redirectsLeft = 5;
  return new Promise((resolve, reject) => {
    const request = https.get(
      url,
      { headers: { 'User-Agent': 'swrm-npm-installer' } },
      (res) => {
        const { statusCode, headers } = res;

        if (statusCode >= 300 && statusCode < 400 && headers.location) {
          res.resume();
          if (redirectsLeft <= 0) {
            reject(new Error(`Too many redirects fetching ${url}`));
            return;
          }
          download(headers.location, destPath, redirectsLeft - 1).then(resolve, reject);
          return;
        }

        if (statusCode !== 200) {
          res.resume();
          reject(new Error(`Download failed with HTTP ${statusCode} for ${url}`));
          return;
        }

        const fileStream = fs.createWriteStream(destPath);
        res.pipe(fileStream);
        fileStream.on('finish', () => fileStream.close(() => resolve()));
        fileStream.on('error', reject);
      }
    );
    request.on('error', reject);
  });
}

async function main() {
  const { os, ext, assetName } = resolveTarget();
  const tag = `v${pkg.version}`;
  const url = `https://github.com/${REPO}/releases/download/${tag}/${assetName}`;

  const distDir = path.join(__dirname, '..', 'dist');
  fs.mkdirSync(distDir, { recursive: true });
  const destPath = path.join(distDir, `swrm${ext}`);

  console.log(`[swrm install] Downloading ${assetName} (${tag})...`);

  try {
    await download(url, destPath);
  } catch (err) {
    try {
      fs.unlinkSync(destPath);
    } catch (_) {
      // nothing to clean up
    }
    fail(
      `Could not download the swrm binary.\n` +
        `  URL:    ${url}\n` +
        `  Reason: ${err.message}\n\n` +
        `  If this keeps happening, build from source: https://github.com/${REPO}#building-from-source`
    );
    return;
  }

  if (os !== 'windows') {
    fs.chmodSync(destPath, 0o755);
  }

  console.log(`[swrm install] Installed to ${destPath}`);
}

main();
