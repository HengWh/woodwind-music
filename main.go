// Command nsgo-music is a native music player for files on disk and the
// sources plugins add, written against NSGo.
package main

import (
	"log"

	"nsgo-music/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
