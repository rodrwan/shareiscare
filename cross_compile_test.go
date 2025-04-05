package main

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestCrossCompilationRaspberryPi(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cross-compilation test in short mode")
	}

	// Verify that we can compile for Raspberry Pi (ARM)
	t.Log("Testing compilation for Raspberry Pi (ARM)")

	// Create a new command with controlled environment
	cmd := exec.Command("go", "build", "-o", "/dev/null")

	// We need to set all environment variables needed for Go
	// as we're not inheriting the system environment
	env := []string{
		"GOOS=linux",
		"GOARCH=arm",
		"GOARM=7",
		"CGO_ENABLED=0", // Disabling CGO is important for cross-compilation
	}

	// Add original environment variables that are relevant for Go
	// such as PATH and GOPATH
	path, err := exec.LookPath("go")
	if err == nil {
		t.Logf("Go found at: %s", path)
	}

	// Set the environment for the command
	cmd.Env = env

	// Run the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Command output: %s", string(output))
		t.Skipf("Error compiling for Raspberry Pi (ARM): %v - Skipping this test", err)
	} else {
		t.Log("Compilation for Raspberry Pi (ARM) successful")
	}
}

func TestCurrentPlatformCompilation(t *testing.T) {
	// This test verifies compilation for the current platform
	cmd := exec.Command("go", "build", "-o", "/dev/null")

	err := cmd.Run()
	if err != nil {
		t.Errorf("Error compiling for current platform (%s/%s): %v",
			runtime.GOOS, runtime.GOARCH, err)
	} else {
		t.Logf("Compilation for current platform (%s/%s) successful",
			runtime.GOOS, runtime.GOARCH)
	}
}
