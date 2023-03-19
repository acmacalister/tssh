//go:build !windows
// +build !windows

package sshproxy

import (
	"fmt"
	"io"
	"net"
	"runtime"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

const (
	tailscaleDevice     = "tailscaleDevice"
	sshContextSSHClient = "sshClient"
	defaultSSHPort      = "22"
)

// sshConn wraps the incoming net.Conn and a cleanup function
// This is done to allow the outgoing SSH client to be retrieved and closed when the conn itself is closed.
type sshConn struct {
	net.Conn
	cleanupFunc func()
}

// close calls the cleanupFunc before closing the conn
func (c sshConn) Close() error {
	c.cleanupFunc()
	return c.Conn.Close()
}

type SSHProxy struct {
	ssh.Server
	hostname  string
	shutdownC chan struct{}
	caCert    ssh.PublicKey
	errorChan chan error
}

// New creates a new SSHProxy and configures its host keys and authentication by the data provided
func New(version, localAddress, hostname, hostKeyDir string, shutdownC chan struct{}, idleTimeout, maxTimeout time.Duration) (*SSHProxy, error) {
	sshProxy := SSHProxy{
		hostname:  hostname,
		shutdownC: shutdownC,
		errorChan: make(chan error),
	}

	sshProxy.Server = ssh.Server{
		Addr:             localAddress,
		MaxTimeout:       maxTimeout,
		IdleTimeout:      idleTimeout,
		Version:          fmt.Sprintf("SSH-2.0-tssh_%s_%s", version, runtime.GOOS),
		PublicKeyHandler: sshProxy.proxyAuthCallback,
		ConnCallback:     sshProxy.connCallback,
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"default": sshProxy.channelHandler,
		},
	}

	return &sshProxy, nil
}

// Start the SSH proxy listener to start handling SSH connections from clients
func (s *SSHProxy) Start() error {
	go func() {
		<-s.shutdownC
		if err := s.Close(); err != nil {
			s.errorChan <- fmt.Errorf("cannot close server: %v", err)
		}
	}()

	return s.ListenAndServe()
}

// Errors return errors from the ssh proxy activities
func (s *SSHProxy) Errors() <-chan error {
	return s.errorChan
}

// proxyAuthCallback attempts to connect to ultimate SSH destination. If successful, it allows the incoming connection
// to connect to the proxy and saves the outgoing SSH client to the context. Otherwise, no connection to the
// the proxy is allowed.
func (s *SSHProxy) proxyAuthCallback(ctx ssh.Context, key ssh.PublicKey) bool {
	client, err := s.dialDestination(ctx)
	if err != nil {
		return false
	}
	ctx.SetValue(sshContextSSHClient, client)
	return true
}

// connCallback reads the preamble sent from the proxy server and saves an audit event logger to the context.
// If any errors occur, the connection is terminated by returning nil from the callback.
func (s *SSHProxy) connCallback(ctx ssh.Context, conn net.Conn) net.Conn {
	// This is a temporary workaround of a timing issue in the tunnel muxer to allow further testing.
	// TODO: Remove this
	time.Sleep(10 * time.Millisecond)

	// TODO: set the tailscale we need to connect to
	ctx.SetValue(tailscaleDevice, "")

	// attempts to retrieve and close the outgoing ssh client when the incoming conn is closed.
	// If no client exists, the conn is being closed before the PublicKeyCallback was called (where the client is created).
	cleanupFunc := func() {
		client, ok := ctx.Value(sshContextSSHClient).(*gossh.Client)
		if ok && client != nil {
			client.Close()
		}
	}

	return sshConn{conn, cleanupFunc}
}

// channelHandler proxies incoming and outgoing SSH traffic back and forth over an SSH Channel
func (s *SSHProxy) channelHandler(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
	if newChan.ChannelType() != "session" && newChan.ChannelType() != "direct-tcpip" {
		msg := fmt.Sprintf("channel type %s is not supported", newChan.ChannelType())
		if err := newChan.Reject(gossh.UnknownChannelType, msg); err != nil {
			s.errorChan <- fmt.Errorf("error rejecting SSH channel: %v", err)
		}
		return
	}

	localChan, localChanReqs, err := newChan.Accept()
	if err != nil {
		s.errorChan <- fmt.Errorf("failed to accept session channel: %v", err)
		return
	}
	defer localChan.Close()

	// client will be closed when the sshConn is closed
	client, ok := ctx.Value(sshContextSSHClient).(*gossh.Client)
	if !ok {
		s.errorChan <- fmt.Errorf("could not retrieve client from context")
		return
	}

	remoteChan, remoteChanReqs, err := client.OpenChannel(newChan.ChannelType(), newChan.ExtraData())
	if err != nil {
		s.errorChan <- fmt.Errorf("failed to open remote channel: %v", err)
		return
	}

	defer remoteChan.Close()

	// Proxy ssh traffic back and forth between client and destination
	s.proxyChannel(localChan, remoteChan, localChanReqs, remoteChanReqs, conn, ctx)
}

// proxyChannel couples two SSH channels and proxies SSH traffic and channel requests back and forth.
func (s *SSHProxy) proxyChannel(localChan, remoteChan gossh.Channel, localChanReqs, remoteChanReqs <-chan *gossh.Request, conn *gossh.ServerConn, ctx ssh.Context) {
	done := make(chan struct{}, 2)
	s.proxyStreams(localChan, remoteChan, done)
	s.proxyStderrStreams(localChan, remoteChan, done)
	s.proxyChannelStreams(localChan, remoteChan, localChanReqs, remoteChan, done)
}

// proxyStreams will proxy the main SSH connection between the clients to the connecting
// tailscale server.
func (s *SSHProxy) proxyStreams(localChan, remoteChan gossh.Channel, done chan struct{}) {
	go func() {
		if _, err := io.Copy(localChan, remoteChan); err != nil {
			s.errorChan <- fmt.Errorf("remote to local copy error: %v", err)
		}
		done <- struct{}{}
	}()
	go func() {
		if _, err := io.Copy(remoteChan, localChan); err != nil {
			s.errorChan <- fmt.Errorf("local to remote copy error: %v", err)
		}
		done <- struct{}{}
	}()
}

// proxyStderrStreams proxies stderr streams.
// These streams are non-pty sessions since they have distinct IO streams.
func (s *SSHProxy) proxyStderrStreams(localChan, remoteChan gossh.Channel, done chan struct{}) {
	remoteStderr := remoteChan.Stderr()
	localStderr := localChan.Stderr()
	go func() {
		if _, err := io.Copy(remoteStderr, localStderr); err != nil {
			s.errorChan <- fmt.Errorf("stderr local to remote copy error: %v", err)
		}
	}()
	go func() {
		if _, err := io.Copy(localStderr, remoteStderr); err != nil {
			s.errorChan <- fmt.Errorf("stderr remote to local copy error: %v", err)
		}
	}()
}

// proxyChannelStreams proxies channel requests. SSH forward channel requests are generally out of band
// to various none PTYs (iirc)
func (s *SSHProxy) proxyChannelStreams(localChan, remoteChan gossh.Channel, localChanReqs, remoteChanReqs <-chan *gossh.Request, done chan struct{}) {
	for {
		select {
		case req := <-localChanReqs:
			if req == nil {
				return
			}
			if err := s.forwardChannelRequest(remoteChan, req); err != nil {
				s.errorChan <- fmt.Errorf("failed to forward request: %v", err)
				return
			}

		case req := <-remoteChanReqs:
			if req == nil {
				return
			}
			if err := s.forwardChannelRequest(localChan, req); err != nil {
				s.errorChan <- fmt.Errorf("failed to forward request: %v", err)
				return
			}
		case <-done:
			return
		}
	}
}

// dialDestination creates a new SSH client and dials the destination server
func (s *SSHProxy) dialDestination(ctx ssh.Context) (*gossh.Client, error) {
	var signer interface{} // := // TODO: Pull signer from client SSH connection

	tailscaleServer, ok := ctx.Value(tailscaleDevice).(string)
	if !ok && tailscaleServer == "" {
		return nil, fmt.Errorf("failed to connect to server")
	}

	clientConfig := &gossh.ClientConfig{
		User:            ctx.User(),
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),                // TODO: respect host keys?
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)}, // need signer from client.
		ClientVersion:   ctx.ServerVersion(),
	}

	client, err := gossh.Dial("tcp", tailscaleServer, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("%v failed to connect to destination SSH server", err)
	}
	return client, nil
}

// forwardChannelRequest sends request req to SSH channel sshChan, waits for reply, and sends the reply back.
func (s *SSHProxy) forwardChannelRequest(sshChan gossh.Channel, req *gossh.Request) error {
	reply, err := sshChan.SendRequest(req.Type, req.WantReply, req.Payload)
	if err != nil {
		return fmt.Errorf("%v failed to send request", err)
	}
	if err := req.Reply(reply, nil); err != nil {
		return fmt.Errorf("%v failed to reply to request", err)
	}
	return nil
}
