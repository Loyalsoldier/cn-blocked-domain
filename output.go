package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

func writeLists(dir string, result lists) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	for _, output := range []struct {
		name  string
		lines []string
	}{
		{"domains.txt", result.domains},
		{"deduplicated-domains.txt", result.removed},
		{"ip.txt", result.prefixes},
	} {
		if err := writeFile(filepath.Join(dir, output.name), output.lines); err != nil {
			return fmt.Errorf("write %s: %w", output.name, err)
		}
	}
	return nil
}

// Replace each list only after it has been completely written.
func writeFile(path string, lines []string) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".cn-blocked-domain-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	writer := bufio.NewWriter(file)
	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	if err := file.Chmod(0644); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
