"""
IngestKit CLI wrapper

This module provides a thin Python wrapper around the IngestKit Go binary.
"""

import os
import platform
import stat
import subprocess
import sys
import shutil
import urllib.request

VERSION = "0.1.8"
BINARY_BASE_URL = "https://github.com/feat7/ingestkit/releases/download/v{version}"


def get_platform_info():
    """Get platform-specific binary name"""
    system = platform.system().lower()
    machine = platform.machine().lower()

    system_map = {
        'darwin': 'darwin',
        'linux': 'linux',
        'windows': 'windows'
    }

    arch_map = {
        'x86_64': 'amd64',
        'amd64': 'amd64',
        'arm64': 'arm64',
        'aarch64': 'arm64',
    }

    system_name = system_map.get(system, system)
    arch_name = arch_map.get(machine, machine)

    binary_name = f"ingestkit-{system_name}-{arch_name}"
    if system == 'windows':
        binary_name += '.exe'

    return binary_name, system_name


def download_binary():
    """Download the IngestKit binary for the current platform"""
    binary_name, system_name = get_platform_info()
    binary_url = f"{BINARY_BASE_URL.format(version=VERSION)}/{binary_name}"

    # Install to ~/.local/bin (user-writable, commonly in PATH)
    install_dir = os.path.expanduser('~/.local/bin')
    os.makedirs(install_dir, exist_ok=True)

    binary_path = os.path.join(install_dir, 'ingestkit-cli')
    if system_name == 'windows':
        binary_path += '.exe'

    print(f"📥 Downloading IngestKit CLI v{VERSION}...")
    print(f"   From: {binary_url}")
    print(f"   To: {binary_path}")
    sys.stdout.flush()

    try:
        urllib.request.urlretrieve(binary_url, binary_path)

        # Make executable (Unix-like systems)
        if system_name != 'windows':
            os.chmod(binary_path, os.stat(binary_path).st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)

        print(f"✅ IngestKit CLI installed successfully!")
        sys.stdout.flush()
        return binary_path
    except Exception as e:
        print(f"❌ Failed to download binary: {e}")
        print(f"\n💡 Manual installation:")
        print(f"   Visit: https://github.com/feat7/ingestkit/releases/latest")
        print(f"   Download: {binary_name}")
        print(f"   Rename to: ingestkit-cli")
        print(f"   Move to: {install_dir}")
        print(f"   Make executable: chmod +x {binary_path}")
        return None


def main():
    """
    Main entry point for the ingestkit CLI command.

    This simply forwards all arguments to the Go binary (ingestkit-cli).
    """
    # Find the ingestkit-cli binary (named differently to avoid conflict with this Python entry point)
    binary = shutil.which('ingestkit-cli')

    # If not found in PATH, check ~/.local/bin directly
    if not binary:
        local_bin = os.path.expanduser('~/.local/bin/ingestkit-cli')
        if os.path.isfile(local_bin) and os.access(local_bin, os.X_OK):
            binary = local_bin

    # Still not found? Download it
    if not binary:
        print("IngestKit CLI binary not found. Installing...")
        sys.stdout.flush()
        binary = download_binary()
        if not binary:
            sys.exit(1)
        print()  # Blank line before running command
        sys.stdout.flush()

    # Forward all arguments to the binary
    args = [binary] + sys.argv[1:]

    # On Unix-like systems, use execvp to replace this process with the binary.
    # This avoids file descriptor inheritance issues (like EAGAIN/Errno 35 on macOS)
    # that can occur when stdin/stdout are in non-blocking mode.
    if hasattr(os, 'execvp'):
        try:
            os.execvp(binary, args)
        except OSError as e:
            print(f"❌ Error running ingestkit: {e}")
            sys.exit(1)
    else:
        # Windows fallback: use subprocess with explicit fd handling
        try:
            result = subprocess.run(
                args,
                stdin=sys.stdin,
                stdout=sys.stdout,
                stderr=sys.stderr,
                check=False
            )
            sys.exit(result.returncode)
        except KeyboardInterrupt:
            sys.exit(130)  # Standard exit code for SIGINT
        except Exception as e:
            print(f"❌ Error running ingestkit: {e}")
            sys.exit(1)


if __name__ == '__main__':
    main()
