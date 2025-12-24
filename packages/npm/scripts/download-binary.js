#!/usr/bin/env node

/**
 * Download IngestKit CLI binary for the current platform
 */

const https = require('https');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const VERSION = '0.1.0';
const BINARY_BASE_URL = `https://github.com/feat7/ingestkit/releases/download/v${VERSION}`;

function getPlatformInfo() {
  const platform = process.platform;
  const arch = process.arch;

  // Map platform names
  const platformMap = {
    'darwin': 'darwin',
    'linux': 'linux',
    'win32': 'windows'
  };

  // Map architecture names
  const archMap = {
    'x64': 'amd64',
    'arm64': 'arm64'
  };

  const systemName = platformMap[platform] || platform;
  const archName = archMap[arch] || arch;

  let binaryName = `ingestkit-${systemName}-${archName}`;
  if (platform === 'win32') {
    binaryName += '.exe';
  }

  return { binaryName, platform: systemName };
}

function downloadBinary() {
  const { binaryName, platform: systemName } = getPlatformInfo();
  const binaryUrl = `${BINARY_BASE_URL}/${binaryName}`;

  // Determine binary path
  const binDir = path.join(__dirname, '..', 'bin');
  const binaryPath = path.join(binDir, systemName === 'windows' ? 'ingestkit.exe' : 'ingestkit');

  console.log('📥 Downloading IngestKit CLI...');
  console.log(`   From: ${binaryUrl}`);
  console.log(`   To: ${binaryPath}`);

  // Ensure bin directory exists
  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }

  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(binaryPath);

    https.get(binaryUrl, (response) => {
      if (response.statusCode === 302 || response.statusCode === 301) {
        // Follow redirect
        https.get(response.headers.location, (redirectResponse) => {
          redirectResponse.pipe(file);
          file.on('finish', () => {
            file.close(() => {
              // Make executable on Unix-like systems
              if (systemName !== 'windows') {
                fs.chmodSync(binaryPath, 0o755);
              }

              console.log('✅ IngestKit CLI installed successfully!');
              console.log('   Run "npx ingestkit --help" to get started');
              resolve();
            });
          });
        }).on('error', reject);
      } else {
        response.pipe(file);
        file.on('finish', () => {
          file.close(() => {
            // Make executable on Unix-like systems
            if (systemName !== 'windows') {
              fs.chmodSync(binaryPath, 0o755);
            }

            console.log('✅ IngestKit CLI installed successfully!');
            console.log('   Run "npx ingestkit --help" to get started');
            resolve();
          });
        });
      }
    }).on('error', (err) => {
      fs.unlink(binaryPath, () => {});  // Delete partial download
      console.error('❌ Failed to download binary:', err.message);
      console.error('\n💡 Manual installation:');
      console.error('   Visit: https://github.com/feat7/ingestkit/releases/latest');
      console.error(`   Download: ${binaryName}`);
      console.error(`   Place in: ${binDir}`);
      reject(err);
    });
  });
}

// Run download
downloadBinary().catch((err) => {
  console.error('Installation failed:', err);
  process.exit(1);
});
