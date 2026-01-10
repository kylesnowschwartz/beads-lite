package main

import (
	"os"

	beadslite "github.com/kylesnowschwartz/beads-lite"
)

func main() {
	if err := beadslite.Run(os.Args[1:], os.Stdout); err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
		os.Exit(1)
	}
}
