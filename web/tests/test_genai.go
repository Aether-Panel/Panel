package main

import (
	"fmt"
	"os"

	"google.golang.org/genai"
)

func main() {
	var resp genai.GenerateContentResponse
	fmt.Fprintf(os.Stderr, "Type: %T\n", resp.Candidates)
}
