"""
IngestKit CLI wrapper

This module provides a thin Python wrapper around the IngestKit Go binary.
"""

import os
import subprocess
import sys
import shutil

def main():
    """
    Main entry point for the ingestkit CLI command.

    This simply forwards all arguments to the Go binary (ingestkit-cli).
    """
    # Find the ingestkit-cli binary (named differently to avoid conflict with this Python entry point)
    binary = shutil.which('ingestkit-cli')

    if not binary:
        print("❌ IngestKit CLI binary not found!")
        print("\nThis might happen if:")
        print("1. The installation didn't complete successfully")
        print("2. The binary directory is not in your PATH")
        print("\nTry reinstalling:")
        print("  pip uninstall ingestkit")
        print("  pip install ingestkit")
        sys.exit(1)

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
