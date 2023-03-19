package main

import (
	"log"
	"os"

	"github.com/acmacalister/tssh/tailscale"
	"github.com/acmacalister/tssh/ui"
)

func main() {
	apiKey := os.Getenv("TAILSCALE_API_KEY")
	tailnet := os.Getenv("TAILSCALE_TAILNET")

	tailscaleService, err := tailscale.New(apiKey, tailnet)
	if err != nil {
		log.Fatalln(err)
	}

	if err := ui.New(tailscaleService); err != nil {
		log.Fatal(err)
	}
}
