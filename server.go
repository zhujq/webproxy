package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/kr/pty"
	"golang.org/x/crypto/ssh"

	"github.com/google/uuid"
	"github.com/zhujq/websocket"
)

const target = "127.0.0.1:2222"

type client struct {
	listenChannel        chan bool // Channel that the client is listening on
	transmitChannel      chan bool // Channel that the client is writing to
	listener             net.Conn  // The thing to write to
	listenerConnected    bool
	transmitter          net.Conn // The thing to listen from
	transmitterConnected bool
}

var connectedClients map[string]client = make(map[string]client)

var (
	defaultShell = "bash" // ssh shell

	authPublicKeys = map[string]string{
		"user": "AAAAC3NzaC1lZDI1NTE5AAAAIADi9ZoVZstck6ELY0EIB863kD4qp5i6DYpQJHkwBiEo",
	}

	// ssh rsa key
	hostKeyBytes = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEoQIBAAKCAQEA4peQ1PUMDTrA8eqQJ32r1vupfZ3grBMwELDdIG3eK5LNd1Mw
iLtNCdOau6d+sBkd4z+qcOC9SLhV9urLsQhOX/m9+TiNeLGtII8x0IRTA8VFbvpH
jbhH/Sc+HWgnpE5FapFcDmdFvsrWQ1npBQg2qd09moZ0/Dshz5xdz//FNHzJ/x94
7CqHFtjXQ5AM58xFhikdTdQecmKDpDn+l+4nRLnaEQBCFnycR5kc9Whx/vFC1Ym7
Nh1H3upUr4aSqbFhZQsdP4sti5BtcrHHp6E5zhksjz3/Za7pG8NkOvDEzGVe1Fxw
BX43MGIoKobF08JP6eTXX6DK155hm4OPuEXMWQIDAQABAoIBADz/JAPPu2DMUihN
RmT7FYkX0fZ4y4RG3geANOaH7Oi56gmXIVeNZB2jEuI1IotxF3SXLOCZ/xpWVP3V
EuQjIkX/yr4OFTdKTRqYsYY6OMapEhnf0ec6llZ1e+kaoqE+WL1pR+iwsDu+CpOy
3mF2ZpCvd+fjDhbgLCfhJffaGFIaS+ED/v7vVTWt7mK1VdhTJosKGdNb6yyJKkXu
dZDf/2WNaaeXVsfFSod+Ob2dy8qVRpe4G5+N7n8M/9w1Tn0tu9jzPqXPBV+LtGOp
WsomwbUh54hnyy9iDaEn+lDHyfce9jyvEUxUyZQ+cST5xmNQUdYwl3HqaEdKkQfM
L6v5lqECgYEA+Mz3XMfk5FtJCBT6S/F/1fhqFLDd0VzLKxVckZ/lTUIi85EkhHPw
gzqCFBgITkQWl9N+M5Lr0Vb2rKHD9ui6VR0YVTRcgDBnBCf27jcBN62eoJNt5ZV4
0etM1a4bOQsFr00rwAooadga16Y70ip074fZUGd4kZSUIQ6wZHmSmLUCgYEA6SYV
4liX4YVVL0NtM9WxXEfS3zUCoKj8vGBrb5cqqGsr95QhTN6maohVgBaGoYEV6cRz
b9g45E5DmQ+hMcJ4joUyfWJ7+US9zMmO529/8QF1zn8stwGs3IxLFw6WpSpGCjJm
JURaNvf8Ax/dUTYa5CJHOuVwt4oaEHS4IOHBH5UCgYEA2W9vwxMjU/r/UWPb9yDg
ouQN+XU09jLNkCKEGvSNlj51gz3Wlzcn+9fXNK5oG9ZflGKOCY6eLv58aBSbyZ5M
sfPSfyxapuEmNriikj9Z/gnq9tTBl4JQ68xjAt+9BNZAKpsb4CJAfXgSxWKPJzZZ
qbik0CMNeNVLu7Q1rimdV30CfxBknSVNFWDF/zdThloerFnQswL+tzCUsTCNlwBB
oL42yuCdibnd7dWPwHNBIjY43VGSfoteqKFk31vjvXHCOrfKpcIrKoxcSPwdL+8V
5+kKMT5TstErTPw04RK989mpH0OYR5ZXOAClbxLJKsaLB1kDD/8UItjE3RBLJKcr
OGkCgYA1DqJVf1vqMUbmrB9PwOBuZia613QgPgipyJOcq0yBjR/2j+zm+5XphmaE
LkMgohXJzlHfMzAWZVUewDmNrg37QvW/q/y8AfhzajsBLCd4+bWwAFv375QJ4US/
U1fg+6iT4wf96a6lIJK4O7jRGAt7PmwEmq2wOpi339k5wVIbPA==
-----END RSA PRIVATE KEY-----`)

	sshServerConfig = &ssh.ServerConfig{
		//ServerVersion:     "SSH-2.0-OpenSSH_7.3p1 Debian-1",
		ServerVersion:    "",
		PasswordCallback: passwordCallback,
		//PublicKeyCallback: publicKeyCallback,
	}
)

var (
	no_auth     = []byte{0x05, 0x00}
	rsp_version = []byte{0x00, 0x00}
	//	with_auth = []byte{0x05, 0x02}
	//	auth_success = []byte{0x05, 0x00}
	//	auth_failed  = []byte{0x05, 0x01}
	gen_failed      = []byte{0x05, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	connect_success = []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
)

func BytesCombine(pBytes ...[]byte) []byte {
	var buffer bytes.Buffer
	for index := 0; index < len(pBytes); index++ {
		buffer.Write(pBytes[index])
	}
	return buffer.Bytes()
}

func handleRequests(reqs <-chan *ssh.Request) {
	for req := range reqs {
		log.Printf("recieved out-of-band request: %+v", req)
	}
}

// pty run
func PtyRun(c *exec.Cmd, tty *os.File) (err error) {
	defer tty.Close()
	c.Stdout = tty
	c.Stdin = tty
	c.Stderr = tty
	c.SysProcAttr = &syscall.SysProcAttr{
		Setctty: true,
		Setsid:  true,
	}
	return c.Start()
}

func handleChannels(chans <-chan ssh.NewChannel) {
	for newChannel := range chans {
		if t := newChannel.ChannelType(); t != "session" {
			newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("could not accept channel (%s)", err)
			continue
		}
		log.Print("creating pty...")
		f, tty, err := pty.Open()
		if err != nil {
			log.Printf("could not start pty (%s)", err)
			continue
		}

		var shell string
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = defaultShell
		}
		go func(in <-chan *ssh.Request) {
			for req := range in {
				ok := false
				switch req.Type {
				case "exec":
					ok = true
					// exec command
					command := string(req.Payload[4 : req.Payload[3]+4])
					log.Println("++++++++", command)
					cmd := exec.Command(shell, []string{"-c", command}...)

					cmd.Stdout = channel
					cmd.Stderr = channel
					cmd.Stdin = channel

					err := cmd.Start()
					if err != nil {
						log.Printf("could not start command (%s)", err)
						continue
					}
					go func() {
						_, err := cmd.Process.Wait()
						if err != nil {
							log.Printf("failed to exit bash (%s)", err)
						}
						time.Sleep(time.Second * 1)
						channel.Close()
						log.Printf("session closed")
					}()
				case "shell":
					cmd := exec.Command(shell)
					cmd.Env = []string{"TERM=xterm"}
					err := PtyRun(cmd, tty)
					if err != nil {
						log.Printf("%s", err)
					}
					var once sync.Once
					close := func() {
						channel.Close()
						log.Printf("session closed")
					}
					go func() {
						io.Copy(channel, f)
						once.Do(close)
					}()

					go func() {
						io.Copy(f, channel)
						once.Do(close)
					}()
					if len(req.Payload) == 0 {
						ok = true
					}
				case "pty-req":
					ok = true
					termLen := req.Payload[3]
					termEnv := string(req.Payload[4 : termLen+4])
					w, h := parseDims(req.Payload[termLen+4:])
					SetWinsize(f.Fd(), w, h)
					log.Printf("pty-req '%s'", termEnv)
				case "window-change":
					w, h := parseDims(req.Payload)
					SetWinsize(f.Fd(), w, h)
					continue //no response
				}

				if !ok {
					log.Printf("declining %s request...", req.Type)
				}

				req.Reply(ok, nil)
			}
		}(requests)
	}
}

func parseDims(b []byte) (uint32, uint32) {
	w := binary.BigEndian.Uint32(b)
	h := binary.BigEndian.Uint32(b[4:])
	return w, h
}

type Winsize struct {
	Height uint16
	Width  uint16
	x      uint16 // unused
	y      uint16 // unused
}

func SetWinsize(fd uintptr, w, h uint32) {
	log.Printf("window resize %dx%d", w, h)
	ws := &Winsize{Width: uint16(w), Height: uint16(h)}
	syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
}

func publicKeyCallback(remoteConn ssh.ConnMetadata, remoteKey ssh.PublicKey) (*ssh.Permissions, error) {
	fmt.Println("Trying to auth user " + remoteConn.User())
	authPublicKey, User := authPublicKeys[remoteConn.User()]
	if !User {
		fmt.Println("User does not exist")
		return nil, errors.New("User does not exist")
	}

	authPublicKeyBytes, err := base64.StdEncoding.DecodeString(authPublicKey)
	if err != nil {
		fmt.Println("Could not base64 decode key")
		return nil, errors.New("Could not base64 decode key")
	}

	parsedAuthPublicKey, err := ssh.ParsePublicKey([]byte(authPublicKeyBytes))
	if err != nil {
		fmt.Println("Could not parse public key")
		return nil, err
	}

	if remoteKey.Type() != parsedAuthPublicKey.Type() {
		fmt.Println("Key types don't match")
		return nil, errors.New("Key types do not match")
	}

	remoteKeyBytes := remoteKey.Marshal()
	authKeyBytes := parsedAuthPublicKey.Marshal()
	if len(remoteKeyBytes) != len(authKeyBytes) {
		fmt.Println("Key lengths don't match")
		return nil, errors.New("Keys do not match")
	}

	keysMatch := true
	for i, b := range remoteKeyBytes {
		if b != authKeyBytes[i] {
			keysMatch = false
		}
	}

	if keysMatch == false {
		fmt.Println("Keys don't match")
		return nil, errors.New("Keys do not match")
	}

	return nil, nil
}

// password callback
func passwordCallback(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	if conn.User() == "admin" && string(password) == "admin" {
		return nil, nil
	}
	return nil, fmt.Errorf("password rejected for %q", conn.User())
}

func bindServer(clientId string) {
	if connectedClients[clientId].listenerConnected && connectedClients[clientId].transmitterConnected {
		log.Println("Two-way connection to client established!")
		log.Println("Client <=|F|=> Proxy <-...-> VPN")

		defer func() {
			connectedClients[clientId].listenChannel <- true
			connectedClients[clientId].transmitChannel <- true
			delete(connectedClients, clientId)
		}()

		serverConn, err := net.Dial("tcp", target)
		if err != nil {
			log.Println("Failed to connect to remote server :/", err)
		}
		log.Println("success to dial" + target)

		defer serverConn.Close()

		wait := make(chan bool)

		go func() {
			_, err = io.Copy(connectedClients[clientId].listener, serverConn)
			if err != nil {
				log.Println("Down Conn Disconnect:", err)
			}

			wait <- true
		}()

		go func() {
			_, err = io.Copy(serverConn, connectedClients[clientId].transmitter)
			if err != nil {
				log.Println("Up Conn Disconnect:", err)
			}

			wait <- true
		}()

		log.Println("Full connection established!")
		log.Println("Client <=|F|=> Proxy <---> VPN")

		<-wait
		log.Println("Connection closed")
	}
}

func lsHandler(w http.ResponseWriter, r *http.Request) {
	ao := &websocket.AcceptOptions{InsecureSkipVerify: true}
	conn, err := websocket.Accept(w, r, ao)
	if err != nil {
		log.Println("webscoket accept err：", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "inner error！")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resolvedId := ""

	_, msg, err := conn.Read(ctx)
	if err != nil {
		log.Println("webscoket read clientid err：", err)
		return
	}
	line := string(msg[:])
	if len(line) > 10 && (line[:10] == "Clientid: " || line[:10] == "clientid: ") {
		resolvedId = line[10:]
		log.Println("Found clientid:" + resolvedId)
	} else {
		log.Println("webscoket read clientid err：", err)
		return
	}

	wait := make(chan bool)

	if _, ok := connectedClients[resolvedId]; !ok {
		connectedClients[resolvedId] = client{}
	}

	currentClient := connectedClients[resolvedId]

	currentClient.listener = websocket.NetConn(ctx, conn, websocket.MessageText)
	currentClient.listenChannel = wait
	currentClient.listenerConnected = true
	connectedClients[resolvedId] = currentClient

	log.Println("Attempting to bind listener")

	go bindServer(resolvedId)

	<-wait

}

func tsHandler(w http.ResponseWriter, r *http.Request) {
	ao := &websocket.AcceptOptions{InsecureSkipVerify: true}
	conn, err := websocket.Accept(w, r, ao)
	if err != nil {
		log.Println("webscoket accept err：", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "inner error！")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resolvedId := ""
	_, msg, err := conn.Read(ctx)
	if err != nil {
		log.Println("webscoket read clientid err：", err)
		return
	}
	line := string(msg[:])

	if len(line) > 10 && (line[:10] == "Clientid: " || line[:10] == "clientid: ") {
		resolvedId = line[10:]
		log.Println("Found clientid:" + resolvedId)
	} else {
		log.Println("webscoket read clientid err：", err)
		return
	}

	wait := make(chan bool)

	if _, ok := connectedClients[resolvedId]; !ok {
		connectedClients[resolvedId] = client{}
	}

	currentClient := connectedClients[resolvedId]

	currentClient.transmitter = websocket.NetConn(ctx, conn, websocket.MessageText)
	currentClient.transmitChannel = wait
	currentClient.transmitterConnected = true

	connectedClients[resolvedId] = currentClient

	log.Println("Attempting to bind transmission")

	go bindServer(resolvedId)

	<-wait

}

func defHandler(w http.ResponseWriter, r *http.Request) {
	ao := &websocket.AcceptOptions{InsecureSkipVerify: true}
	conn, err := websocket.Accept(w, r, ao)
	if err != nil {
		log.Println("webscoket accept err：", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "inner error！")
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	b := [1024]byte{0x0}
	netconn := websocket.NetConn(ctx, conn, websocket.MessageBinary)
	n, err := netconn.Read(b[:])
	//	log.Println(n)
	if n < 24 {
		log.Println("Err length of less head")
		return
	}
	if err != nil {
		log.Println("Error reading from websocket conn,error is: ", err)
		return
	}

	if b[0] != 0x00 {
		netconn.Close()
		log.Println("Wrong vless version")
		return
	}
	uuid, err := uuid.FromBytes(b[1:17])
	if err != nil {
		log.Println("Error get uuid,error is: ", err)
		return
	}
	if uuid.String() != "b831381d-6324-4d53-ad4f-8cda48b30811" {
		log.Println("Error uuid")
		return
	}
	//	log.Println("uuid is:" + uuid.String())
	adlen := int(b[17])
	cmd := int(b[18+adlen])
	if cmd != 1 {
		log.Println("Only support TCP")
		return
	}

	var port uint16
	binary.Read(bytes.NewReader(b[(19+adlen):(21+adlen)]), binary.BigEndian, &port)

	addtype := int(b[21+adlen])

	host := ""
	hostlen := 0
	nIndex := 0
	switch addtype {
	case 1:
		host = net.IPv4(b[22+adlen], b[23+adlen], b[24+adlen], b[25+adlen]).String()
		hostlen = 4
		nIndex = 25 + adlen
	case 2:
		hostlen = int(b[(22 + adlen)])
		host = string(b[(23 + adlen):(23 + adlen + hostlen)])
		nIndex = 23 + adlen + hostlen
	case 3:
		hostlen = 16
		host = net.IP{b[22+adlen], b[23+adlen], b[24+adlen], b[25+adlen], b[26+adlen], b[27+adlen], b[28+adlen], b[29+adlen], b[30+adlen], b[31+adlen], b[32+adlen], b[33+adlen], b[34+adlen], b[35+adlen], b[36+adlen], b[37+adlen]}.String()
		nIndex = 37 + adlen
	}
	log.Println(host + ":" + strconv.Itoa(int(port)))

	server, err := net.Dial("tcp", host+":"+strconv.Itoa(int(port)))
	if server != nil {
		defer server.Close()
	}
	if err != nil {
		log.Println("error to connect destination server,error is:", err)
		return
	}
	_, err = server.Write(b[nIndex:n])
	if err != nil {
		log.Println("error to write packet to destination server,error is:", err)
		return
	}

	netconn.Write(rsp_version)
	go io.Copy(server, netconn)
	io.Copy(netconn, server)

	return
}

func main() {
	log.Println("The WS Service is starting...")

	var IPAddress, Port string = "0.0.0.0", "2222"
	hostKey, err := ssh.ParsePrivateKey(hostKeyBytes)
	if err != nil {
		log.Fatal("Failed to parse host key")
	}

	sshServerConfig.AddHostKey(hostKey)

	listener, err := net.Listen("tcp4", IPAddress+":"+Port)
	if err != nil {
		log.Fatalf("failed to listen on %s:%s", IPAddress, Port)
	}

	log.Printf("SSH Service is listening on %s:%s", IPAddress, Port)

	go func() {
		for {
			tcpConn, err := listener.Accept()
			if err != nil {
				log.Printf("failed to accept incoming connection (%s)", err)
				continue
			}
			go func() {
				sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, sshServerConfig)
				if err != nil {
					log.Printf("failed to handshake (%s)", err)
					return
				}

				log.Printf("new connection from %s (%s)", sshConn.RemoteAddr(), sshConn.ClientVersion())
				go handleRequests(reqs)
				go handleChannels(chans)
			}()
		}
	}()

	http.HandleFunc("/listen", lsHandler)
	http.HandleFunc("/transmit", tsHandler)
	http.HandleFunc("/", defHandler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8888"
	}
	port = ":" + port
	log.Fatal(http.ListenAndServe(port, nil))
}
