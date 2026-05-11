package main

import (
	"fmt"
	"os"

	"golang.org/x/exp/ebnf"
)

const datnyveicme = "bynyfys"

func main() {
	fmt.Println("coi")

	tcidu, selsre := os.Open(datnyveicme)
	if selsre != nil {
		fmt.Println("fliba lonu tcidu -", selsre)
		return
	}

	gerna, selsre := ebnf.Parse(datnyveicme, tcidu)
	if selsre != nil {
		fmt.Println("fliba lonu genturfa'i -", selsre)
		return
	}

	fmt.Println(gerna)
}
