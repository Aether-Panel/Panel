package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mholt/archives"
)

// testArchiveName is the archive this test writes and reads back
const testArchiveName = "test_transfer.tar.gz"

func main() {
	err := os.MkdirAll("test_server_dir/some_folder", 0755)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("test_server_dir/file1.txt", []byte("hello world"), 0644)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("test_server_dir/some_folder/file2.txt", []byte("hello world 2"), 0644)
	if err != nil {
		panic(err)
	}

	filenames := map[string]string{
		"test_server_dir": ".",
	}

	ctx := context.Background()
	filesList, err := archives.FilesFromDisk(ctx, nil, filenames)
	if err != nil {
		panic(err)
	}

	out, err := os.Create(testArchiveName)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	format, _, err := archives.Identify(ctx, testArchiveName, nil)
	if err != nil {
		panic(err)
	}

	archiver := format.(archives.Archiver)
	err = archiver.Archive(ctx, out, filesList)
	if err != nil {
		panic(err)
	}

	info, _ := os.Stat(testArchiveName)
	fmt.Printf("Archive size: %d bytes\n", info.Size())
}
