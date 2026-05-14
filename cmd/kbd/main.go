package main

import (
	"os"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/localapp"
)

func main() {
	os.Exit(localapp.App{}.Run(os.Args[1:]))
}
