#!/usr/bin/env node
'use strict';

const path = require('path');
const { spawnSync } = require('child_process');

const binName = process.platform === 'win32' ? 'swrm.exe' : 'swrm';
const binPath = path.join(__dirname, '..', 'dist', binName);

// stdio: 'inherit' hands the child the parent's actual stdin/stdout/stderr
// file descriptors, so the Bubble Tea TUI gets a real TTY — raw keyboard
// input, terminal resize events, and full-speed output — with no piping
// overhead in between.
const result = spawnSync(binPath, process.argv.slice(2), { stdio: 'inherit' });

if (result.error) {
  if (result.error.code === 'ENOENT') {
    console.error(
      `[swrm] Could not find the swrm binary at:\n  ${binPath}\n\n` +
        'Try reinstalling: npm install -g swrm'
    );
  } else {
    console.error(`[swrm] Failed to launch: ${result.error.message}`);
  }
  process.exit(1);
}

if (result.signal) {
  // Killed by a signal (e.g. Ctrl+C forwarded at the OS level) rather than
  // exiting normally — there's no meaningful exit code to forward.
  process.exit(1);
}

process.exit(result.status === null ? 1 : result.status);
