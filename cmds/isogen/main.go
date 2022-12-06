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
	if len(os.Args) < 5 {
		os.Exit(-1)
	}
	f, err := os.CreateTemp("/var/tmp", "immune.*.vfat")
	if err != nil {
		fmt.Printf("failed to create temporary vfat file: %v", err)
		os.Exit(-1)
	}
	f.Close()
	os.RemoveAll(f.Name())

	if err := isogen.Mkvfat(f.Name(), *linuxboot, *shim, *mmx); err != nil {
		fmt.Printf("failed to make vfat partition: %v", err)
		os.Exit(-1)
	}
	if err := isogen.Mkiso(*out, f.Name()); err != nil {
		fmt.Printf("failed to make iso: %v", err)
		os.Exit(-1)
	}
}
