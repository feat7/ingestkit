"""
IngestKit CLI wrapper

This module provides a thin Python wrapper around the IngestKit Go binary.
"""

import subprocess
import sys
import shutil

def main():
    """
    Main entry point for the ingestkit CLI command.

    This simply forwards all arguments to the Go binary.
    """
    # Find the ingestkit binary
    binary = shutil.which('ingestkit')

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
    try:
        result = subprocess.run([binary] + sys.argv[1:], check=False)
        sys.exit(result.returncode)
    except KeyboardInterrupt:
        sys.exit(130)  # Standard exit code for SIGINT
    except Exception as e:
        print(f"❌ Error running ingestkit: {e}")
        sys.exit(1)

if __name__ == '__main__':
    main()
