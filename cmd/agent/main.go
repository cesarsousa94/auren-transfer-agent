package main

import (
	"fmt"
	"os"

	"github.com/cesarsousa94/auren-transfer-agent/internal/bootstrap"
)

func main() {
	if err := bootstrap.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "auren-transfer-agent: %v\n", err)
		os.Exit(1)
	}
}
