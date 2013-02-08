package util

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ReadLines(r io.Reader) []string {
	buf := bufio.NewReader(r)
	lines := make([]string, 0)
	for {
		line, err := buf.ReadString('\n')
		if err != nil && err != io.EOF {
			Fatalf("Could not read line: %s.", err)
		}
		lines = append(lines, strings.TrimSpace(line))
		if err == io.EOF {
			break
		}
	}
	return lines
}

func CopyFile(src, dest string) {
	_, err := io.Copy(CreateFile(dest), OpenFile(src))
	Assert(err, "Could not copy '%s' to '%s'", src, dest)
}

func IsDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func RecursiveFiles(dir string) []string {
	files := make([]string, 0)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			Warnf("Could not read '%s' because: %s\n", path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files
}
