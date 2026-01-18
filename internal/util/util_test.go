package util

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPtr(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		s := "test"
		p := Ptr(s)
		if *p != s {
			t.Errorf("expected %q, got %q", s, *p)
		}
	})

	t.Run("int", func(t *testing.T) {
		i := 42
		p := Ptr(i)
		if *p != i {
			t.Errorf("expected %d, got %d", i, *p)
		}
	})

	t.Run("int64", func(t *testing.T) {
		i := int64(123)
		p := Ptr(i)
		if *p != i {
			t.Errorf("expected %d, got %d", i, *p)
		}
	})
}

func TestEnsureDirectoryWritable(t *testing.T) {
	t.Run("creates non-existent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		newDir := filepath.Join(tmpDir, "new", "nested", "dir")

		err := EnsureDirectoryWritable(newDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		info, err := os.Stat(newDir)
		if err != nil {
			t.Fatalf("directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected directory")
		}
	})

	t.Run("existing writable directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := EnsureDirectoryWritable(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("file instead of directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "file.txt")
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		err := EnsureDirectoryWritable(filePath)
		if err == nil {
			t.Error("expected error for file path")
		}
	})
}

func TestWithRetry(t *testing.T) {
	t.Run("all succeed first try", func(t *testing.T) {
		items := []int{1, 2, 3}
		callCount := 0

		failed := WithRetry(items, 3, func(i int) error {
			callCount++
			return nil
		})

		if len(failed) != 0 {
			t.Errorf("expected no failures, got %d", len(failed))
		}
		if callCount != 3 {
			t.Errorf("expected 3 calls, got %d", callCount)
		}
	})

	t.Run("all fail", func(t *testing.T) {
		items := []int{1, 2}

		failed := WithRetry(items, 2, func(i int) error {
			return errors.New("always fail")
		})

		if len(failed) != 2 {
			t.Errorf("expected 2 failures, got %d", len(failed))
		}
	})

	t.Run("partial failure then success", func(t *testing.T) {
		items := []int{1, 2, 3}
		attempts := make(map[int]int)

		failed := WithRetry(items, 3, func(i int) error {
			attempts[i]++
			if i == 2 && attempts[i] < 2 {
				return errors.New("fail first time")
			}
			return nil
		})

		if len(failed) != 0 {
			t.Errorf("expected no failures, got %d", len(failed))
		}
		if attempts[2] != 2 {
			t.Errorf("expected item 2 to be tried twice, got %d", attempts[2])
		}
	})

	t.Run("empty items", func(t *testing.T) {
		var items []int

		failed := WithRetry(items, 3, func(i int) error {
			return errors.New("should not be called")
		})

		if len(failed) != 0 {
			t.Errorf("expected no failures, got %d", len(failed))
		}
	})

	t.Run("zero retries", func(t *testing.T) {
		items := []int{1}
		callCount := 0

		failed := WithRetry(items, 0, func(i int) error {
			callCount++
			return errors.New("fail")
		})

		if len(failed) != 1 {
			t.Errorf("expected 1 failure, got %d", len(failed))
		}
		if callCount != 1 {
			t.Errorf("expected 1 call, got %d", callCount)
		}
	})
}

func TestGetOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		value    *string
		def      string
		expected string
	}{
		{
			name:     "nil pointer returns default",
			value:    nil,
			def:      "default",
			expected: "default",
		},
		{
			name:     "empty string returns default",
			value:    Ptr(""),
			def:      "default",
			expected: "default",
		},
		{
			name:     "non-empty string returns value",
			value:    Ptr("actual"),
			def:      "default",
			expected: "actual",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetOrDefault(tt.value, tt.def)
			if result != tt.expected {
				t.Errorf("GetOrDefault() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetOrDefaultInt(t *testing.T) {
	tests := []struct {
		name     string
		value    *int
		def      int
		expected int
	}{
		{
			name:     "nil pointer returns default",
			value:    nil,
			def:      42,
			expected: 42,
		},
		{
			name:     "zero value returns zero",
			value:    Ptr(0),
			def:      42,
			expected: 0,
		},
		{
			name:     "non-zero value returns value",
			value:    Ptr(100),
			def:      42,
			expected: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetOrDefaultInt(tt.value, tt.def)
			if result != tt.expected {
				t.Errorf("GetOrDefaultInt() = %d, want %d", result, tt.expected)
			}
		})
	}
}
