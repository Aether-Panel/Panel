package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mholt/archives"
)

func main() {
	prefix := "test_server_dir"
	err := os.MkdirAll(prefix, 0755)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(filepath.Join(prefix, "file1.txt"), []byte("hello world"), 0644)
	if err != nil {
		panic(err)
	}

	targetFile := filepath.Join(prefix, "transfer.tar.gz")

	filesToCompress := []string{prefix}

	filenames := make(map[string]string)
	for _, f := range filesToCompress {
		rel, _ := filepath.Rel(prefix, f)
		rel = filepath.ToSlash(rel)
		filenames[f] = rel
	}

	fmt.Printf("filenames map: %v\n", filenames)

	ctx := context.Background()
	filesList, err := archives.FilesFromDisk(ctx, nil, filenames)
	if err != nil {
		panic(err)
	}

	fmt.Printf("filesList len: %d\n", len(filesList))
	for _, f := range filesList {
		fmt.Printf(" - %s\n", f.NameInArchive)
	}

	out, err := os.Create(targetFile)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	format, _, err := archives.Identify(ctx, targetFile, nil)
	if err != nil {
		panic(err)
	}

	archiver := format.(archives.Archiver)
	err = archiver.Archive(ctx, out, filesList)
	if err != nil {
		panic(err)
	}

	info, _ := os.Stat(targetFile)
	fmt.Printf("Archive size: %d bytes\n", info.Size())
}
