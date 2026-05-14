package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/hidapp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(hidapp.App{Context: ctx}.Run(os.Args[1:]))
}
