package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"runtime/debug"
//	"os"
)

const port = "8080"
const target = "127.0.0.1:22"

type client struct {
	listenChannel        chan bool // Channel that the client is listening on
	transmitChannel      chan bool // Channel that the client is writing to
	listener             io.Writer // The thing to write to
	listenerConnected    bool
	transmitter          io.Reader // The thing to listen from
	transmitterConnected bool
}

var connectedClients map[string]client = make(map[string]client)

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
			_, err := io.Copy(connectedClients[clientId].listener, serverConn)
			if err != nil {
				log.Println("Disconnect:", err)
			}

			wait <- true
		}()

		go func() {
			_, err := io.Copy(serverConn, connectedClients[clientId].transmitter)
			if err != nil {
				log.Println("Disconnect:", err)
			}

			wait <- true
		}()

		log.Println("Full connection established!")
		log.Println("Client <=|F|=> Proxy <---> VPN")

		<-wait
		log.Println("Connection closed")
	}
}

func handleConnection(clientConn net.Conn) {
	defer clientConn.Close()
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Connection panic:", r)
			debug.PrintStack()
		}
	}()

	reader := bufio.NewReader(clientConn)

	line, err := reader.ReadString('\n')
	if err != nil {
		// log.Println("Failed to read first line", err)
		return
	}
	if line == "GET /listen HTTP/1.1\r\n" {
		// This is for LISTENING
		resolvedId := ""
		for line, err = reader.ReadString('\n'); true; line, err = reader.ReadString('\n') {
			if err != nil {
				// log.Println("Failed to read following lines", err)
				return
			}

			if len(line) > 10 && line[:10] == "Clientid: " {
				resolvedId = line[10:30]
			}

			if line == "\r\n" {
				break
			}
		}

		if len(resolvedId) > 1 {
                        log.Println("success to get resolvedid:" + resolvedId)

			fmt.Fprintf(clientConn, "HTTP/1.1 101 Switching Protocols\r\n")
			fmt.Fprintf(clientConn, "Upgrade: websocket\r\n")
			fmt.Fprintf(clientConn, "Connection: Upgrade\r\n")
			fmt.Fprintf(clientConn, "Content-Type: application/octet-stream\r\n")
			fmt.Fprintf(clientConn, "Connection: keep-alive\r\n")
			fmt.Fprintf(clientConn, "Content-Length: 12345789000\r\n\r\n")

			wait := make(chan bool)

			if _, ok := connectedClients[resolvedId]; !ok {
				connectedClients[resolvedId] = client{}
			}

			currentClient := connectedClients[resolvedId]

			currentClient.listener = clientConn
			currentClient.listenChannel = wait
			currentClient.listenerConnected = true

			connectedClients[resolvedId] = currentClient

			log.Println("Attempting to bind listener")

			go bindServer(resolvedId)

			<-wait
		} else {
		 	 log.Println("Failed to find client id!")
		}

	} else if line == "GET /transmit HTTP/1.1\r\n" {
		// This is for TRANSMITTING
		log.Println("start to receive post init")
	
		resolvedId := ""
		for line, err = reader.ReadString('\n'); true; line, err = reader.ReadString('\n') {
			if err != nil {
				log.Println("Failed to read following lines")
				return
			}

			if len(line) > 10 && line[:10] == "Clientid: " {
				resolvedId = line[10:30]
			}

			if line == "\r\n" {
				break
			}
		}

		if len(resolvedId) > 1 {

		
			fmt.Fprintf(clientConn, "HTTP/1.1 101 Switching Protocols\r\n")
                        fmt.Fprintf(clientConn, "Upgrade: websocket\r\n")
                        fmt.Fprintf(clientConn, "Connection: Upgrade\r\n")

			fmt.Fprintf(clientConn, "Content-Type: application/octet-stream\r\n")
			fmt.Fprintf(clientConn, "Connection: keep-alive\r\n")
			fmt.Fprintf(clientConn, "Content-Length: 12345798000\r\n\r\n")
			wait := make(chan bool)

			if _, ok := connectedClients[resolvedId]; !ok {
				connectedClients[resolvedId] = client{}
			}

			currentClient := connectedClients[resolvedId]

			currentClient.transmitter = reader
			currentClient.transmitChannel = wait
			currentClient.transmitterConnected = true

			connectedClients[resolvedId] = currentClient

			log.Println("Attempting to bind transmission")

			go bindServer(resolvedId)

			<-wait
		} else {
			log.Println("Failed to find client id!")
		}

	} else {
		fmt.Fprintf(clientConn, "HTTP/1.1 404 Not found\r\n")
		fmt.Fprintf(clientConn, "Content-Type: text/plain\r\n")
		fmt.Fprintf(clientConn, "Content-Length: 8\r\n\r\n")
		fmt.Fprintf(clientConn, "u wot m9")
	}
}

func main() {
	log.Println("Listening...")
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Println("Error listening!", err)
		return
	}
//	log.Println(os.Getenv("QOVERY_BRANCH_NAME"))
//	log.Println(os.Getenv("QOVERY_APPLICATION_WEBPROXY_HOSTNAME"))
	for true {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Error accepting connection", err)
			continue
		}

		go handleConnection(conn)
	}
}
