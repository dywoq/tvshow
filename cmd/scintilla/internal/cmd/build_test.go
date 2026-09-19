package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"tvshow/lang/interpreter"
)

func TestBuildCommand(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "main.sc")
	destFile := filepath.Join(tempDir, "out.scb")

	sourceContent := `
		int Add(int a, int b) { return a + b; }
		int Start() { return Add(20, 22); }
	`
	if err := os.WriteFile(srcFile, []byte(sourceContent), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	cmd := Build()
	cmd.SetArgs([]string{"--source", srcFile, "--destination", destFile})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Build command failed: %v", err)
	}

	data, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("Failed to read output bytecode file: %v", err)
	}

	interp, err := interpreter.NewFromBinary(data)
	if err != nil {
		t.Fatalf("interpreter.NewFromBinary failed: %v", err)
	}

	got, err := interp.Run("Start")
	if err != nil {
		t.Fatalf("interp.Run failed: %v", err)
	}

	if got != int64(42) {
		t.Errorf("interp.Run = %v, want 42", got)
	}
}
