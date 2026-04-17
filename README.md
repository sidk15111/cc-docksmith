# 🛠️ Docksmith

![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?style=flat&logo=go)
![Linux Requirement](https://img.shields.io/badge/Platform-Linux-FCC624?style=flat&logo=linux)
![Status](https://img.shields.io/badge/Status-Complete-success)

**Docksmith** is a custom-built, lightweight, offline-first container engine written entirely in Go. It replicates the core primitives of modern containerization—Linux Namespaces, `chroot`, Union File Systems (OverlayFS), and deterministic layer caching—without relying on external daemons like Docker or containerd.

This project was built from scratch to demonstrate a deep understanding of how container engines interact directly with the Linux kernel.

---

## ✨ Features

- **Custom AST Parser:** Reads and executes instructions from a custom `Docksmithfile` (supports `FROM`, `WORKDIR`, `ENV`, `COPY`, `RUN`, `CMD`).
- **Deterministic Build Cache:** Implements a cascading SHA-256 hash system for layers. If an instruction and its environment haven't changed, Docksmith instantly loads the cached `.tar` layer instead of rebuilding.
- **Kernel-Level Isolation:** Utilizes Linux Namespaces (`CLONE_NEWPID`, `CLONE_NEWUTS`, `CLONE_NEWNS`) and `chroot` to create a strictly sandboxed child process blind to the host system.
- **Union Filesystem (OverlayFS):** Mounts immutable lower directories and a writable upper directory to capture exactly what changed during a `RUN` command without duplicating data.
- **Standalone Execution:** The compiled binary is entirely self-referential (`os.Executable()`). It does not require the Go compiler to be installed on the host machine to spawn isolated child processes.
- **OCI-Style Manifests:** Tracks image metadata, environment variables, and layer digests via standard JSON files.

---

## 🏗️ Architecture: How it Works

Docksmith is divided into two distinct boundaries: the **Host (Orchestrator)** and the **Child (Sandbox)**.

1. **The Build Phase:** The engine parses the `Docksmithfile` and processes instructions step-by-step. For static commands (`ENV`, `WORKDIR`), it updates internal configuration. For execution commands (`RUN`), it mounts a temporary OverlayFS and spawns itself as an isolated child process to execute the command. 
2. **Delta Capture:** After a `RUN` command completes, the orchestrator inspects the OverlayFS `upperdir`, packages the changed files into an immutable `.tar` tarball, and records its SHA-256 digest in the manifest.
3. **The Run Phase:** When booting a container, Docksmith stacks all required layer tarballs via OverlayFS, provisions a virtual `/proc` filesystem, injects configured environment variables, and drops the user into an interactive, locked-down shell.

---

## 🚀 Getting Started

### Prerequisites
- A **Linux** environment (or a Linux VM/WSL2). Docksmith relies on Linux-specific system calls (`unshare`, `mount`, `chroot`).
- `sudo` privileges (required to mount OverlayFS and configure namespaces).
- `wget` and `tar` installed on the host.

### Installation & Initialization

Do not build the Go binary manually on your first run. Use the provided `bootstrap.sh` script to set up the environment, download the Alpine base minirootfs, hash it, and compile the engine.

```bash
# Clone the repository
git clone [https://github.com/yourusername/docksmith.git](https://github.com/yourusername/docksmith.git)
cd docksmith

# Make the bootstrap script executable
chmod +x bootstrap.sh

# Run the initialization
./bootstrap.sh
