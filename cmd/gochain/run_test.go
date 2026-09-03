package main

import (
	"bytes"
	"fmt"
	"testing"
)

func TestRunHelp(t *testing.T) {
	tests := []struct {
		name string
		arg  string
	}{
		{name: "help command", arg: "help"},
		{name: "short flag", arg: "-h"},
		{name: "long flag", arg: "--help"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			exitCode := run([]string{tt.arg}, &stdout, &stderr)

			if exitCode != 0 {
				t.Fatalf("exit code = %d, want 0", exitCode)
			}
			if stdout.String() != usage {
				t.Errorf("stdout = %q, want %q", stdout.String(), usage)
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want no output", stderr.String())
			}
		})
	}
}

func TestRunWithoutCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(nil, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want no output", stdout.String())
	}
	if stderr.String() != usage {
		t.Errorf("stderr = %q, want %q", stderr.String(), usage)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"dance"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want no output", stdout.String())
	}
	wantStderr := fmt.Sprintf("unknown command %q\n\n%s", "dance", usage)
	if stderr.String() != wantStderr {
		t.Errorf("stderr = %q, want %q", stderr.String(), wantStderr)
	}
}
