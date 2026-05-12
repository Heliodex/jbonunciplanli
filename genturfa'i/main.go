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

	for cmene, genjva := range gerna {
		mekso := genjva.Expr

		fmt.Println(cmene, ":", mekso)
		switch mekso.(type) {
		case ebnf.Alternative:
			fmt.Println("ebnf.Alternative")
		case ebnf.Sequence:
			fmt.Println("ebnf.Sequence")
		case *ebnf.Name:
			fmt.Println("*ebnf.Name")
		case *ebnf.Token:
			fmt.Println("*ebnf.Token")
		case *ebnf.Range:
			fmt.Println("*ebnf.Range")
		case *ebnf.Group:
			fmt.Println("*ebnf.Group")
		case *ebnf.Option:
			fmt.Println("*ebnf.Option")
		case *ebnf.Repetition:
			fmt.Println("*ebnf.Repetition")
		case *ebnf.Production:
			fmt.Println("*ebnf.Production")
		case *ebnf.Bad:
			fmt.Println("*ebnf.Bad")
		}
	}
}
