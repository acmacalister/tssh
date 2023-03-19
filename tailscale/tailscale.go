package tailscale

import (
	"context"

	"github.com/acmacalister/tssh"
	"github.com/tailscale/tailscale-client-go/tailscale"
)

type service struct {
	client *tailscale.Client
}

func New(apiKey, tailnet string) (tssh.TailscaleService, error) {
	client, err := tailscale.NewClient(apiKey, tailnet)
	if err != nil {
		return nil, err
	}
	return &service{client: client}, nil
}

func (s *service) Devices() ([]tailscale.Device, error) {
	return s.client.Devices(context.Background())
}
