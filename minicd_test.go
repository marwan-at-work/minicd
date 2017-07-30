package minicd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCloneRepo(t *testing.T) {
	t.Parallel()
	tempDir, err := cloneRepo("", "https://github.com/marwan-at-work/minicd.git", "80f9eef19f0294447a144c0e7c5ab845d4b836c7")
	if err != nil {
		t.Fatal(err)
	}

	err = os.RemoveAll(tempDir)
	if err != nil {
		t.Errorf("could not remove repo directory at: %v", tempDir)
	}
}

func TestCompilePkg(t *testing.T) {
	t.Parallel()

	cwd := getWd(t)
	path := filepath.Join(cwd, "test_data", "server")
	err := compilePkg(path)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCp(t *testing.T) {
	t.Parallel()

	cwd := getWd(t)
	src := filepath.Join(cwd, "test_data", "cp", "a", "sample-file.txt")
	dst := filepath.Join(cwd, "test_data", "cp", "b")
	fullDstPath := filepath.Join(dst, "sample-file.txt")
	err := os.RemoveAll(fullDstPath)
	if err != nil {
		t.Fatal(err)
	}

	err = cp(src, dst)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = os.Stat(fullDstPath); os.IsNotExist(err) {
		t.Fatal("expected sample-file.txt to be copied to folder b")
	}
}

func getWd(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	return cwd
}
