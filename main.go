package main

import (
	"log"
	"os"

	"whatsappgui-new/internal/profile"
	"whatsappgui-new/internal/workspace"
)

func main() {
	var err error
	if len(os.Args) == 3 && os.Args[1] == "--profile" {
		err = profile.Run(os.Args[2])
	} else {
		err = workspace.Run()
	}
	if err != nil {
		log.Fatal(err)
	}
}
