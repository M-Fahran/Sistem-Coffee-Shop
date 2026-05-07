package main

import (
	"log"
	"os"
	"coffeeshop/cmd/bootstrap"
)

func main() {
	if err := bootstrap.Run(); err != nil {
		log.Printf("application error: %v", err)
		os.Exit(1)
	}
}