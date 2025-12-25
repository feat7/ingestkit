"""
IngestKit Python Package

This package provides a wrapper around the IngestKit Go binary
for easy installation and usage in Python projects.
"""

from setuptools import setup, find_packages
from setuptools.command.install import install
from setuptools.command.develop import develop
import platform
import urllib.request
import os
import stat
import sys

VERSION = "0.1.7"
BINARY_BASE_URL = "https://github.com/feat7/ingestkit/releases/download/v{version}"

def get_platform_info():
    """Get platform-specific binary name"""
    system = platform.system().lower()
    machine = platform.machine().lower()

    # Map platform names
    system_map = {
        'darwin': 'darwin',
        'linux': 'linux',
        'windows': 'windows'
    }

    # Map architecture names
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

    # Determine installation directory
    if '--user' in sys.argv:
        # User install
        install_dir = os.path.expanduser('~/.local/bin')
    else:
        # System install (requires sudo)
        if system_name == 'windows':
            install_dir = os.path.join(os.environ.get('LOCALAPPDATA', ''), 'ingestkit', 'bin')
        else:
            install_dir = '/usr/local/bin'

    os.makedirs(install_dir, exist_ok=True)

    # Use a different name for the Go binary to avoid conflicts with Python entry point
    binary_path = os.path.join(install_dir, 'ingestkit-cli')
    if system_name == 'windows':
        binary_path += '.exe'

    print(f"📥 Downloading IngestKit CLI from {binary_url}...")
    print(f"   Installing to: {binary_path}")

    try:
        urllib.request.urlretrieve(binary_url, binary_path)

        # Make executable (Unix-like systems)
        if system_name != 'windows':
            os.chmod(binary_path, os.stat(binary_path).st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)

        print(f"✅ IngestKit CLI installed successfully!")
        print(f"   Run 'ingestkit --help' to get started")

        return binary_path
    except Exception as e:
        print(f"❌ Failed to download binary: {e}")
        print(f"\n💡 Manual installation:")
        print(f"   Visit: https://github.com/feat7/ingestkit/releases/latest")
        print(f"   Download: {binary_name}")
        print(f"   Rename to: ingestkit-cli")
        print(f"   Move to: {install_dir}")
        raise

def is_build_environment():
    """Check if we're in a build environment (not actual user install)"""
    # Check common build environment indicators
    venv = os.environ.get('VIRTUAL_ENV', '')
    if 'build-env' in venv or '/tmp/' in venv:
        return True
    # Check if running wheel/sdist build commands
    build_cmds = ['bdist_wheel', 'sdist', 'build', 'egg_info']
    if any(cmd in sys.argv for cmd in build_cmds):
        return True
    return False

class PostInstallCommand(install):
    """Post-installation hook to download binary"""
    def run(self):
        install.run(self)
        # Only download during actual pip install, not during wheel build
        if not is_build_environment():
            try:
                download_binary()
            except Exception as e:
                # Don't fail installation if binary download fails
                print(f"⚠️  Binary download skipped: {e}")
                print("   Run 'ingestkit' to trigger download later")

class PostDevelopCommand(develop):
    """Post-develop hook to download binary"""
    def run(self):
        develop.run(self)
        if not is_build_environment():
            try:
                download_binary()
            except Exception as e:
                print(f"⚠️  Binary download skipped: {e}")
                print("   Run 'ingestkit' to trigger download later")

# Read long description from README
long_description = """
# IngestKit Python Package

Easy event ingestion for Python applications.

## Installation

```bash
pip install ingestkit
```

## Quick Start

```bash
# Initialize in your project
ingestkit init

# Define events in ingestkit/schema.yaml

# Generate type-safe client
ingestkit generate

# Use in your code
from ingestkit import Client

client = Client()
client.user_signup.send(user_id="123", email="user@example.com")
```

## What's Included

- `ingestkit` CLI command
- Auto-generated type-safe Python client
- Pydantic models with validation
- Simple configuration management

## Requirements

- Python 3.7+
- Internet connection (for initial binary download)

## Documentation

Visit: https://github.com/feat7/ingestkit

## License

Apache 2.0
"""

setup(
    name='ingestkit',
    version=VERSION,
    description='High-performance event ingestion platform with type-safe SDKs',
    long_description=long_description,
    long_description_content_type='text/markdown',
    author='IngestKit Team',
    author_email='hello@ingestkit.dev',
    url='https://github.com/feat7/ingestkit',
    packages=find_packages(),
    install_requires=[
        'requests>=2.25.0',
        'pydantic>=1.8.0',
    ],
    python_requires='>=3.7',
    entry_points={
        'console_scripts': [
            'ingestkit=ingestkit.cli:main',
        ],
    },
    cmdclass={
        'install': PostInstallCommand,
        'develop': PostDevelopCommand,
    },
    classifiers=[
        'Development Status :: 4 - Beta',
        'Intended Audience :: Developers',
        'License :: OSI Approved :: Apache Software License',
        'Programming Language :: Python :: 3',
        'Programming Language :: Python :: 3.7',
        'Programming Language :: Python :: 3.8',
        'Programming Language :: Python :: 3.9',
        'Programming Language :: Python :: 3.10',
        'Programming Language :: Python :: 3.11',
        'Topic :: Software Development :: Libraries',
        'Topic :: System :: Monitoring',
    ],
    keywords='events analytics tracking ingestion observability',
)
