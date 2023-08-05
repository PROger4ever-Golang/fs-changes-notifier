package main

import (
	"log"

	"fs-changes-notifier/commands"
)

func main() {
	if err := commands.GetCommand().Execute(); err != nil {
		log.Fatal(err)
	}
}
