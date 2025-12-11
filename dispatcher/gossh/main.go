package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"gossh/internal/auth"
	"gossh/internal/connector"
	"gossh/internal/handler"
	"gossh/internal/logger"
	"gossh/internal/resource"

	"golang.org/x/crypto/ssh"
)

const (
	host = "0.0.0.0"
	port = 22
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	privateKey, err := loadOrGenerateHostKey("/certs/ssh_host_rsa_key")
	if err != nil {
		log.Fatalf("Failed to load host key: %v", err)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		log.Fatalf("Failed to listen on %s:%d: %v", host, port, err)
	}
	defer resource.CloseListener(listener)

	log.Printf("SSH Proxy listening on %s:%d", host, port)

	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go handleClient(tcpConn, privateKey)
	}
}

func handleClient(tcpConn net.Conn, hostKey ssh.Signer) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in handleClient: %v", r)
		}
	}()

	addr := tcpConn.RemoteAddr().String()
	log.Printf("Connection from %s", addr)

	authenticator := auth.NewAuthenticator("./config/user.txt")
	// heraldingConnector := connector.NewSSHConnector("heralding", 22)
	cowrieConnector := connector.NewSSHConnector("cowrie", 2222)

	var username, password string

	config := &ssh.ServerConfig{
		NoClientAuth: false,
		PasswordCallback: func(conn ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			username = conn.User()
			password = string(pass)

			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Recovered from panic in record_login: %v", r)
					}
				}()
				_ = cowrieConnector.RecordLogin(username, password)
			}()

			authSuccess := authenticator.Authenticate(username, password)
			logger.LogAuthEvent(addr, fmt.Sprintf("%s:%d", host, port), username, password, authSuccess)

			if authSuccess {
				return nil, nil
			}
			return nil, fmt.Errorf("authentication failed")
		},
	}

	config.AddHostKey(hostKey)

	sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, config)
	if err != nil {
		if err != io.EOF {
			log.Printf("SSH handshake failed: %v", err)
		}
		resource.CloseSocket(tcpConn)
		return
	}

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("Failed to accept channel: %v", err)
			continue
		}

		go func(ch ssh.Channel, reqs <-chan *ssh.Request) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from panic in session handler: %v", r)
				}
				resource.CloseChannel(ch)
			}()

			var sessionStarted bool
			for req := range reqs {
				switch req.Type {
				case "pty-req":
					width, height := parsePtyRequest(req.Payload)
					cowrieConnector.UpdateTerminalSize(int(width), int(height))
					req.Reply(true, nil)

				case "window-change":
					width, height := parseWindowChange(req.Payload)
					cowrieConnector.UpdateTerminalSize(int(width), int(height))
					if req.WantReply {
						req.Reply(true, nil)
					}

				case "shell":
					req.Reply(true, nil)
					if !sessionStarted {
						sessionStarted = true
						startTime := time.Now()
						handler.HandleSession(
							ch,
							reqs,
							username,
							password,
							addr,
							startTime,
							cowrieConnector,
							sshConn,
							tcpConn,
						)
					}
					return

				case "exec":
					req.Reply(true, nil)

				default:
					if req.WantReply {
						req.Reply(false, nil)
					}
				}
			}
		}(channel, requests)
	}

	resource.CloseConnection(sshConn)
	resource.CloseSocket(tcpConn)
}

func parsePtyRequest(payload []byte) (width, height uint32) {
	if len(payload) < 12 {
		return 80, 24
	}

	termNameLen := binary.BigEndian.Uint32(payload[0:4])
	offset := 4 + termNameLen

	if len(payload) < int(offset)+8 {
		return 80, 24
	}

	width = binary.BigEndian.Uint32(payload[offset : offset+4])
	height = binary.BigEndian.Uint32(payload[offset+4 : offset+8])

	return
}

func parseWindowChange(payload []byte) (width, height uint32) {
	if len(payload) < 8 {
		return 80, 24
	}
	width = binary.BigEndian.Uint32(payload[0:4])
	height = binary.BigEndian.Uint32(payload[4:8])
	return
}

func loadOrGenerateHostKey(keyPath string) (ssh.Signer, error) {
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, err
		}

		privateKeyPEM := &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		}

		keyFile, err := os.Create(keyPath)
		if err != nil {
			return nil, err
		}
		defer resource.CloseFile(keyFile)

		if err := pem.Encode(keyFile, privateKeyPEM); err != nil {
			return nil, err
		}

		keyBytes = pem.EncodeToMemory(privateKeyPEM)
	}

	return ssh.ParsePrivateKey(keyBytes)
}
