// Command sizeprobe renders one mark and writes it to standard output.
//
// It exists to be weighed, not to be run. A library's cost to its callers is
// the code it drags into their binary, and the only honest way to measure
// that is to build the smallest possible program that uses it. Everything
// here beyond the blazon call — one os.Stdout.Write, no flags, no fmt — is
// deliberately absent so that what the scale reads is this library.
//
// The size budget in the tinygo CI job is measured on this program. PNG is
// the format probed because it is the widest path through the library:
// entropy, rasteriser, and encoder.
package main

import (
	"os"

	"github.com/timzifer/blazon"
)

func main() {
	version := "1.4.2"
	if len(os.Args) > 1 {
		version = os.Args[1]
	}
	data, err := blazon.PNG(version, blazon.Options{Size: 256})
	if err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	os.Stdout.Write(data)
}
