package resource

import (
	"io"
	"log"
	"net"

	"golang.org/x/crypto/ssh"
)

func CloseChannel(channel ssh.Channel) {
	if channel == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic while closing channel: %v", r)
		}
	}()

	channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
	channel.Close()
}

func CloseConnection(conn ssh.Conn) {
	if conn == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic while closing connection: %v", r)
		}
	}()

	conn.Close()
}

func CloseSocket(conn net.Conn) {
	if conn == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic while closing socket: %v", r)
		}
	}()

	conn.Close()
}

func CloseSession(session interface{ Close() error }) {
	if session == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic while closing session: %v", r)
		}
	}()

	session.Close()
}

func CloseClient(client *ssh.Client) {
	if client == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic while closing client: %v", r)
		}
	}()

	client.Close()
}

func CloseFile(file io.Closer) {
	if file == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic while closing file: %v", r)
		}
	}()

	file.Close()
}

func CloseListener(listener net.Listener) {
	if listener == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic while closing listener: %v", r)
		}
	}()

	listener.Close()
}

func CloseResponseBody(body io.Closer) {
	if body == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic while closing response body: %v", r)
		}
	}()

	body.Close()
}

func CloseSSHSession(client *ssh.Client, session interface{ Close() error }) {
	CloseSession(session)
	CloseClient(client)
}

func CloseFullSSHConnection(channel ssh.Channel, conn ssh.Conn, tcpConn net.Conn) {
	CloseChannel(channel)
	CloseConnection(conn)
	CloseSocket(tcpConn)
}

func CloseClientAndSession(client *ssh.Client, session interface{ Close() error }) {
	CloseSession(session)
	CloseClient(client)
}
