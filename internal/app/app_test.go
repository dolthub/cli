package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestMain(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
		wantErr  []string
	}{
		{
			name:     "help",
			args:     []string{"--help"},
			wantCode: 0,
			wantOut:  "DoltHub from the command line",
		},
		{
			name:     "version",
			args:     []string{"version"},
			wantCode: 0,
			wantOut:  "dh version test\n",
		},
		{
			name:     "unknown command",
			args:     []string{"unknown"},
			wantCode: 1,
			wantErr:  []string{`unknown command "unknown"`, "Usage:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Main(tt.args, strings.NewReader(""), &stdout, &stderr, "test")

			if code != tt.wantCode {
				t.Fatalf("Main() code = %d, want %d", code, tt.wantCode)
			}
			if tt.wantOut != "" && !strings.Contains(stdout.String(), tt.wantOut) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantOut)
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr = %q, want it to contain %q", stderr.String(), want)
				}
			}
		})
	}
}
