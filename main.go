// idle-timeout - Kill a process if no stdout/stderr output for a specified duration
//
// Usage: idle-timeout <duration> <command> [args...]
// Example: idle-timeout 30s curl -s https://example.com
//          idle-timeout 300 crush run "my prompt"
//
// Exit codes:
//   - 124: Process killed due to inactivity timeout
//   - Otherwise: Exit code of the wrapped command

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// parseDuration parses a duration string, defaulting to seconds if no unit
func parseDuration(s string) (time.Duration, error) {
	if secs, err := strconv.ParseFloat(s, 64); err == nil {
		return time.Duration(secs * float64(time.Second)), nil
	}
	return time.ParseDuration(s)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: idle-timeout <duration> <command> [args...]\n")
		fmt.Fprintf(os.Stderr, "Example: idle-timeout 30s mycommand arg1 arg2\n")
		os.Exit(1)
	}

	timeout, err := parseDuration(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid duration %q: %v\n", os.Args[1], err)
		fmt.Fprintf(os.Stderr, "Examples: 30, 30s, 1m, 2m30s\n")
		os.Exit(1)
	}

	cmdName := os.Args[2]
	cmdArgs := os.Args[3:]

	exitCode := run(cmdName, cmdArgs, timeout)
	os.Exit(exitCode)
}

func run(cmdName string, cmdArgs []string, timeout time.Duration) int {
	// Print spawn line like expect does
	fmt.Printf("spawn %s", cmdName)
	for _, arg := range cmdArgs {
		fmt.Printf(" %s", arg)
	}
	fmt.Println()

	// Build the command string for script
	cmdStr := cmdName
	for _, arg := range cmdArgs {
		// Quote arguments that contain spaces
		if strings.ContainsAny(arg, " \t\n'\"") {
			cmdStr += " " + "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
		} else {
			cmdStr += " " + arg
		}
	}

	// Use 'script' command for perfect TTY emulation
	// -q = quiet, -c = command, /dev/null = don't save typescript
	cmd := exec.Command("script", "-q", "-c", cmdStr, "/dev/null")
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create stdout pipe: %v\n", err)
		return 1
	}

	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start command: %v\n", err)
		return 1
	}

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		if cmd.Process != nil {
			cmd.Process.Signal(sig.(syscall.Signal))
		}
	}()

	// Activity tracker
	var mu sync.Mutex
	lastActivity := time.Now()

	resetTimer := func() {
		mu.Lock()
		lastActivity = time.Now()
		mu.Unlock()
	}

	// Timeout checker
	done := make(chan struct{})
	timedOut := false

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mu.Lock()
				elapsed := time.Since(lastActivity)
				mu.Unlock()

				if elapsed >= timeout {
					timedOut = true
					fmt.Fprintf(os.Stderr, "\n[idle-timeout] No output for %v, killing process...\n", timeout)
					if cmd.Process != nil {
						cmd.Process.Kill()
					}
					return
				}
			}
		}
	}()

	// Read output byte-by-byte for real-time display
	buf := make([]byte, 4096)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			resetTimer()
			os.Stdout.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	// Wait for command to finish
	err = cmd.Wait()
	close(done)

	if timedOut {
		return 124
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}

	return 0
}
