package main

import (
	"os"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/rpiapp"
)

func main() {
	os.Exit(rpiapp.App{}.Run(os.Args[1:]))
}
