package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type Config struct {
	RemoteServer string
	RemotePort   int
	LocalServer  string
	LocalPort    int
	BindPort     int
	ID           string
	KeepAlive    bool
}

type BoreClient struct {
	config         Config
	sshConfig      *ssh.ClientConfig
	sshClient      *ssh.Client
	LocalEndpoint  endpoint
	ServerEndpoint endpoint
	RemoteEndpoint endpoint
	id             string
}

type idRequestPayload struct {
	ID string
}

// NewBoreClient — აბრუნებს pointer-ს და error-ს
func NewBoreClient(config Config) (*BoreClient, error) {
	if config.RemoteServer == "" {
		return nil, fmt.Errorf("RemoteServer არ არის მითითებული")
	}
	if config.RemotePort == 0 {
		return nil, fmt.Errorf("RemotePort არ არის მითითებული")
	}
	return &BoreClient{
		config:         config,
		LocalEndpoint:  endpoint{config.LocalServer, config.LocalPort},
		ServerEndpoint: endpoint{config.RemoteServer, config.RemotePort},
		RemoteEndpoint: endpoint{"0.0.0.0", config.BindPort},
		sshConfig:      &ssh.ClientConfig{HostKeyCallback: ssh.InsecureIgnoreHostKey()},
		id:             config.ID,
	}, nil
}

// boreListener — sync.Once პანიკის თავიდან ასაცილებლად
type boreListener struct {
	ch     chan net.Conn
	once   sync.Once
	closed chan struct{}
}

func newBoreListener() *boreListener {
	return &boreListener{
		ch:     make(chan net.Conn, 32),
		closed: make(chan struct{}),
	}
}

func (l *boreListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.ch:
		return conn, nil
	case <-l.closed:
		return nil, fmt.Errorf("bore: listener closed")
	}
}

func (l *boreListener) Close() error {
	l.once.Do(func() { close(l.closed) }) // sync.Once — double-close პანიკი აღარ იქნება
	return nil
}

func (l *boreListener) Addr() net.Addr {
	return &net.TCPAddr{}
}

// Listen — context-ს იღებს cancellation-ისთვის
func (c *BoreClient) Listen(ctx context.Context) (net.Listener, error) {
	sshClient, err := ssh.Dial("tcp", c.ServerEndpoint.String(), c.sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH კავშირის შეცდომა: %w", err)
	}
	c.sshClient = sshClient

	keepaliveDone := make(chan struct{})
	if c.config.KeepAlive {
		go keepAliveTicker(c.sshClient, keepaliveDone)
	}

	if c.id != "" {
		_, _, err = c.sshClient.SendRequest("set-id", true, ssh.Marshal(&idRequestPayload{c.id}))
		if err != nil {
			return nil, fmt.Errorf("ID set-ის შეცდომა: %w", err)
		}
	}

	if err := c.writeStdout(); err != nil {
		return nil, err
	}

	sshListener, err := c.sshClient.Listen("tcp", c.RemoteEndpoint.String())
	if err != nil {
		return nil, fmt.Errorf("SSH listener შეცდომა: %w", err)
	}

	bl := newBoreListener()

	// ctx cancel → listener-ების დახურვა
	go func() {
		<-ctx.Done()
		bl.Close()
		sshListener.Close()
	}()

	go func() {
		defer sshListener.Close()
		defer close(keepaliveDone)
		defer bl.Close()

		for {
			remote, err := sshListener.Accept()
			if err != nil {
				return
			}

			server, local := net.Pipe()
			bl.ch <- server
			go handleClient(remote, local)
		}
	}()

	return bl, nil
}

func (c *BoreClient) writeStdout() error {
	session, err := c.sshClient.NewSession()
	if err != nil {
		return err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}

	go func() {
		defer session.Close()
		io.Copy(os.Stdout, stdout)
	}()

	return nil
}

type endpoint struct {
	host string
	port int
}

func (e *endpoint) String() string {
	return fmt.Sprintf("%s:%d", e.host, e.port)
}

func handleClient(a net.Conn, b net.Conn) {
	defer a.Close()
	defer b.Close()

	done := make(chan struct{}, 2)
	go func() { io.Copy(a, b); done <- struct{}{} }()
	go func() { io.Copy(b, a); done <- struct{}{} }()
	<-done
}

func keepAliveTicker(client *ssh.Client, done <-chan struct{}) error {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			_, _, err := client.SendRequest("keepalive", true, nil)
			if err != nil {
				return err
			}
		case <-done:
			return nil
		}
	}
}
