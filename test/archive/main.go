package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mholt/archives"
)

func main() {
	err := os.MkdirAll("test_server_dir/some_folder", 0755)
	if err != nil {
		panic(err)
	}
	os.WriteFile("test_server_dir/file1.txt", []byte("hello world"), 0644)
	os.WriteFile("test_server_dir/some_folder/file2.txt", []byte("hello world 2"), 0644)

	filenames := map[string]string{
		"test_server_dir": ".",
	}

	ctx := context.Background()
	filesList, err := archives.FilesFromDisk(ctx, nil, filenames)
	if err != nil {
		panic(err)
	}

	out, err := os.Create("test_transfer.tar.gz")
	if err != nil {
		panic(err)
	}
	defer out.Close()

	format, _, err := archives.Identify(ctx, "test_transfer.tar.gz", nil)
	if err != nil {
		panic(err)
	}

	archiver := format.(archives.Archiver)
	err = archiver.Archive(ctx, out, filesList)
	if err != nil {
		panic(err)
	}

	info, _ := os.Stat("test_transfer.tar.gz")
	fmt.Printf("Archive size: %d bytes\n", info.Size())
}
