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
// Child is the actual isolated process running INSIDE the namespaces.
func Child() error {
	// The new contract expects: child <rootfs> <workdir> <cmd> <args...>
	// os.Args[0] is the program name
	// os.Args[1] is "child"
	// os.Args[2] is the rootfs path
	// os.Args[3] is the working directory
	// os.Args[4] is the command (e.g., /bin/sh)
	// os.Args[5:] are the command arguments (e.g., -c, echo "Testing")
	if len(os.Args) < 5 {
		return fmt.Errorf("invalid child arguments, expected at least 5 but got %d", len(os.Args))
	}

	rootfs := os.Args[2]
	workdir := os.Args[3]
	command := os.Args[4]
	cmdArgs := os.Args[5:]

	// 1. Hostname Isolation (Optional but good practice)
	syscall.Sethostname([]byte("docksmith-container"))

	// 2. Lock the container to the rootfs
	if err := syscall.Chroot(rootfs); err != nil {
		return fmt.Errorf("chroot failed: %v", err)
	}

	// 3. Move into the requested Working Directory (e.g., /app)
	// If the folder doesn't exist in the base image, create it!
	os.MkdirAll(workdir, 0755)
	if err := os.Chdir(workdir); err != nil {
		return fmt.Errorf("chdir failed: %v", err)
	}
	//this did not work
	// This is required for many basic Linux tools to work
	os.MkdirAll("/proc", 0755)
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		// We don't want to crash if it's already mounted, but it's good to know
		fmt.Printf("Warning: proc mount failed: %v\n", err)
	}

	// 2. Clear the Mount on exit (Crucial!)
	defer syscall.Unmount("/proc", 0)

	// 4. Find the executable inside the new isolated filesystem
	execPath, err := exec.LookPath(command)
	if err != nil {
		return fmt.Errorf("command not found inside container: %v", err)
	}

	// 5. Assemble the final arguments array (Linux requires the executable name to be arg 0)
	finalArgs := append([]string{command}, cmdArgs...)

	// 6. Replace the docksmith child process entirely with the target command (e.g., /bin/sh)
	if err := syscall.Exec(execPath, finalArgs, os.Environ()); err != nil {
		return fmt.Errorf("exec failed: %v", err)
	}

	return nil
}
