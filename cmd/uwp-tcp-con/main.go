package main

import (
	"fmt"
	"os"

	"UWP-TCP-Con/internal/cli"
)

func main() {
	app := cli.NewApp()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
