package main

import (
	"fmt"
	"os"
)

func main() {
	base := "http://localhost:8080"
	if v := os.Getenv("SIMPLE_CHAT_URL"); v != "" {
		base = v
	}
	fmt.Printf("simple-chat go client (stub); target %s — see ../../protocol.md\n", base)
	os.Exit(0)
}
