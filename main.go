package main

import (
	"fmt"
	"os"

	"main/game"
)

func main() {
	g := game.New()
	if err := g.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
