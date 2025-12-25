#!/usr/bin/env node

/**
 * IngestKit CLI wrapper for Node.js
 *
 * This script forwards all arguments to the IngestKit Go binary.
 */

const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

// Determine binary path
const platform = process.platform;
const binaryName = platform === 'win32' ? 'ingestkit.exe' : 'ingestkit';
const binaryPath = path.join(__dirname, binaryName);

async function ensureBinary() {
  // Check if binary exists
  if (fs.existsSync(binaryPath)) {
    return binaryPath;
  }

  // Binary not found, try to download it
  console.log('IngestKit CLI binary not found. Installing...');

  try {
    const { downloadBinary } = require('../scripts/download-binary.js');
    await downloadBinary();
    console.log();  // Blank line before running command
    return binaryPath;
  } catch (err) {
    console.error('❌ Failed to download IngestKit CLI binary');
    console.error('\nThis might happen if:');
    console.error('1. The installation didn\'t complete successfully');
    console.error('2. You\'re using an unsupported platform');
    console.error('3. Network issues prevented the download');
    console.error('\nTry reinstalling:');
    console.error('  npm uninstall ingestkit');
    console.error('  npm install ingestkit');
    process.exit(1);
  }
}

async function main() {
  await ensureBinary();

  // Forward all arguments to the binary
  const child = spawn(binaryPath, process.argv.slice(2), {
    stdio: 'inherit'
  });

  // Handle exit
  child.on('close', (code) => {
    process.exit(code || 0);
  });

  // Handle errors
  child.on('error', (err) => {
    console.error('❌ Error running ingestkit:', err.message);
    process.exit(1);
  });
}

main();
