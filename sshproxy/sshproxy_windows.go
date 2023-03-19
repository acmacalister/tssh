//go:build windows
// +build windows

package sshproxy

import (
	"errors"
	"time"
)

type SSHServer struct{}

func New(_, _, _, _ string, _ chan struct{}, _, _ time.Duration) (*SSHServer, error) {
	return nil, errors.New("ssh proxy is not supported on windows")
}

func (s *SSHServer) Start() error {
	return errors.New("ssh proxy is not supported on windows")
}
