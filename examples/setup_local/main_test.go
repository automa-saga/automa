package main

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestMainIntegration(t *testing.T) {
	// Only run on supported OS
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("Integration test only runs on linux/darwin")
	}

	// Redirect stdout/stderr
	stdout := os.Stdout
	stderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	// Run main
	done := make(chan struct{})
	go func() {
		main()
		wOut.Close()
		wErr.Close()
		close(done)
	}()

	<-done

	// Restore stdout/stderr
	os.Stdout = stdout
	os.Stderr = stderr

	// Read output
	var bufOut, bufErr bytes.Buffer
	bufOut.ReadFrom(rOut)
	bufErr.ReadFrom(rErr)

	// Check for expected output
	outStr := bufOut.String() + bufErr.String()
	fmt.Println(outStr)

	if !strings.Contains(outStr, fmt.Sprintf("%s setup_directory", greenTick)) {
		t.Errorf("Expected ✔ setup_directory")
	}
	if !strings.Contains(outStr, fmt.Sprintf("%s install_task", greenTick)) {
		t.Errorf("Expected ✔ install_task")
	}
	if !strings.Contains(outStr, "✘ install_kind") {
		t.Errorf("Expected ✘ install_kind")
	}
	if !strings.Contains(outStr, "Finished workflow") {
		t.Errorf("Expected workflow completion messages")
	}
	if !strings.Contains(outStr, "Workflow Report") {
		t.Errorf("Expected workflow report in output")
	}
}
