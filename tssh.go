package tssh

import "github.com/tailscale/tailscale-client-go/tailscale"

type Action int

const (
	ActionNone Action = iota
	ActionSSH
	ActionDeviceSSH
)

type TailscaleService interface {
	Devices() ([]tailscale.Device, error)
}
