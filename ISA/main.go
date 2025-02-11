package main

import (
	"os"
)

func main() {
	// mostra os arquivos de entrada
	for i := 1; i < len(os.Args); i++ {
		println(os.Args[i])
	}

	// Abrindo os arquivos de entrada e saida
	// var input string = os.Args[1]
	// var output string = os.Args[2]
}
