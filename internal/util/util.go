package util

import (
	"fmt"
	"log"
	"os"
	"path"
	"time"
)

func Ptr[T any](v T) *T {
	return &v
}

func EnsureDirectoryWritable(dir string) error {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	testFile := path.Join(dir, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return err
	}
	_ = f.Close()
	_ = os.Remove(testFile)

	return nil
}

func WithRetry[T any](items []T, maxRetries int, fn func(T) error) []T {
	pending := items
	for attempt := 0; attempt <= maxRetries && len(pending) > 0; attempt++ {
		if attempt > 0 {
			log.Printf("retrying %d failed items (attempt %d/%d)", len(pending), attempt, maxRetries)
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		var failed []T
		for _, item := range pending {
			if err := fn(item); err != nil {
				failed = append(failed, item)
			}
		}
		pending = failed
	}
	return pending
}

func GetOrDefault(v *string, d string) string {
	if v == nil || *v == "" {
		return d
	}
	return *v
}
