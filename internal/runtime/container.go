package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Run sets up Linux namespaces and re-executes the docksmith binary inside them.
func Run(rootfs string, workdir string, env []string, cmdArgs []string) error {
	// /proc/self/exe is a Linux magic link pointing to the currently running docksmith binary.
	// We are telling the binary to run ITSELF, but pass "child" as the first argument.
	args := append([]string{"child", rootfs, workdir}, cmdArgs...)
	cmd := exec.Command("/proc/self/exe", args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	// This is the core isolation. We are asking the kernel to create new Namespaces:
	// CLONE_NEWUTS: New Hostname isolation
	// CLONE_NEWPID: New Process ID isolation (the container process becomes PID 1)
	// CLONE_NEWNS:  New Mount isolation (so we don't mess up the host's mounts)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS,
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("container exited with error: %v", err)
	}
	return nil
}

// Child is the actual isolated process running INSIDE the namespaces.
func Child() error {
	if len(os.Args) < 5 {
		return fmt.Errorf("child process requires rootfs, workdir, and command")
	}

	rootfs := os.Args[2]
	workdir := os.Args[3]
	cmdArgs := os.Args[4:]

	// 1. Chroot locks the filesystem to our target directory
	if err := syscall.Chroot(rootfs); err != nil {
		return fmt.Errorf("chroot failed (are you running as root?): %v", err)
	}

	// 2. Change into the requested working directory
	if err := os.Chdir(workdir); err != nil {
		return fmt.Errorf("chdir failed: %v", err)
	}

	// 3. Find the executable inside the new isolated filesystem
	execPath, err := exec.LookPath(cmdArgs[0])
	if err != nil {
		return fmt.Errorf("command not found inside container: %v", err)
	}

	// 4. Replace the docksmith child process entirely with the target command (e.g., /bin/sh)
	if err := syscall.Exec(execPath, cmdArgs, os.Environ()); err != nil {
		return fmt.Errorf("exec failed: %v", err)
	}

	return nil
}
