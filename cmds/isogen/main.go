package main

import (
	"flag"
	"fmt"
	"os"

	isogen "github.com/immune-gmbh/isogen/pkg"
)

func main() {
	linuxboot := flag.String("linuxboot", "", "a string")
	shim := flag.String("shim", "", "a string")
	mmx := flag.String("mmx", "", "a string")
	out := flag.String("out", "", "a string")
	flag.Parse()
	if len(os.Args) < 4 {
		os.Exit(-1)
	}
	if err := isogen.MkEFIBootloader(*out, *linuxboot, *shim, *mmx); err != nil {
		fmt.Printf("failed to make efi bootloader iso: %v", err)
		os.Exit(-1)
	}
}
