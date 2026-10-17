package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"genturfa_i/lojbo"
)

const datnyveicme = "bynyfys pa"

// gernaDatnyvei finds the grammar whether the program runs inside the
// genturfa'i directory or from the repository root.
func gernaDatnyvei(name string) string {
	if _, err := os.Stat(name); err == nil {
		return name
	}
	alt := filepath.Join("genturfa'i", name)
	if _, err := os.Stat(alt); err == nil {
		return alt
	}
	return name
}

func main() {
	var (
		gernaCmene = flag.String("g", datnyveicme, "datnyvei be fi lo gerna")
		datnyvei   = flag.String("f", "", "datnyvei be lo jufra")
		xisku      = flag.Bool("x", false, "jarco lo gerna stura")
	)
	flag.Parse()

	gerna, selsre := lojbo.Load(gernaDatnyvei(*gernaCmene))
	if selsre != nil {
		fmt.Fprintln(os.Stderr, selsre)
		os.Exit(1)
	}

	var tcidu []byte
	switch {
	case *datnyvei != "":
		var selsre error
		tcidu, selsre = os.ReadFile(*datnyvei)
		if selsre != nil {
			fmt.Fprintln(os.Stderr, "fliba lonu tcidu -", selsre)
			os.Exit(1)
		}
	case flag.NArg() > 0:
		tcidu = []byte(strings.Join(flag.Args(), " "))
	default:
		var selsre error
		tcidu, selsre = io.ReadAll(os.Stdin)
		if selsre != nil {
			fmt.Fprintln(os.Stderr, "fliba lonu tcidu -", selsre)
			os.Exit(1)
		}
	}

	stura, selsre := gerna.Parse(string(tcidu))
	if selsre != nil {
		fmt.Fprintln(os.Stderr, selsre)
		os.Exit(1)
	}

	if *xisku {
		fmt.Print(stura)
	} else {
		fmt.Println("je'u")
	}
}
