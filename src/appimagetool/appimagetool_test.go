package main

import "testing"

func TestGenerateAppImage(t *testing.T) {
	type args struct {
		appdir string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestNormalizePathToUsrPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lib to usr/lib",
			input:    "/lib/x86_64-linux-gnu/libfoo.so",
			expected: "/usr/lib/x86_64-linux-gnu/libfoo.so",
		},
		{
			name:     "lib64 to usr/lib64",
			input:    "/lib64/libbar.so",
			expected: "/usr/lib64/libbar.so",
		},
		{
			name:     "bin to usr/bin",
			input:    "/bin/myapp",
			expected: "/usr/bin/myapp",
		},
		{
			name:     "sbin to usr/sbin",
			input:    "/sbin/mytool",
			expected: "/usr/sbin/mytool",
		},
		{
			name:     "usr/lib unchanged",
			input:    "/usr/lib/x86_64-linux-gnu/libqux.so",
			expected: "/usr/lib/x86_64-linux-gnu/libqux.so",
		},
		{
			name:     "usr/bin unchanged",
			input:    "/usr/bin/otherapp",
			expected: "/usr/bin/otherapp",
		},
		{
			name:     "opt unchanged",
			input:    "/opt/myapp/lib/libfoo.so",
			expected: "/opt/myapp/lib/libfoo.so",
		},
		{
			name:     "nested lib path",
			input:    "/lib/x86_64-linux-gnu/gdk-pixbuf-2.0/loaders/libfoo.so",
			expected: "/usr/lib/x86_64-linux-gnu/gdk-pixbuf-2.0/loaders/libfoo.so",
		},
		{
			name:     "local lib unchanged",
			input:    "/usr/local/lib/libfoo.so",
			expected: "/usr/local/lib/libfoo.so",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePathToUsrPrefix(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePathToUsrPrefix(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
