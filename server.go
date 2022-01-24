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

const port = "80"
const target = "127.0.0.1:22"
const v2proxy = "127.0.0.1:8080"

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
				log.Println("Down Conn Disconnect:", err)
			}

			wait <- true
		}()

		go func() {
			_, err := io.Copy(serverConn, connectedClients[clientId].transmitter)
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
		//		log.Println("Failed to read first line", err)
		return
	}
	if line == "GET /listen HTTP/1.1\r\n" {
		// This is for LISTENING

		resolvedId := ""
		for line, err = reader.ReadString('\n'); true; line, err = reader.ReadString('\n') {
			if err != nil {
				log.Println("Failed to read following lines", err)
				return
			}
			log.Println(line)

			if len(line) > 10 && (line[:10] == "Clientid: " || line[:10] == "clientid: ") { //2021-10-05增加，cloudflare 会把http头名改成小写
				log.Println("Found clientid!")
				resolvedId = line[10:30]
				log.Println(resolvedId)
			}

			if line == "\r\n" {
				break
			}
		}

		if len(resolvedId) > 1 {
			log.Println("success to get resolvedid:" + resolvedId)

			fmt.Fprintf(clientConn, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nContent-Type: application/octet-stream\r\nConnection: keep-alive\r\n\r\n")
			/*	fmt.Fprintf(clientConn, "Upgrade: websocket\r\n")
					fmt.Fprintf(clientConn, "Connection: Upgrade\r\n")
					fmt.Fprintf(clientConn, "Content-Type: application/octet-stream\r\n")
					fmt.Fprintf(clientConn, "Connection: keep-alive\r\n")
					fmt.Fprintf(clientConn, "Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=\r\n")
				//	fmt.Fprintf(clientConn, "Content-Encoding: gzip\r\n")
					fmt.Fprintf(clientConn, "Strict-Transport-Security: max-age=15724800; includeSubDomains\r\n")
					fmt.Fprintf(clientConn, "Transfer-Encoding: chunked\r\n\r\n")

				//	fmt.Fprintf(clientConn, "Content-Length: 999999\r\n\r\n")
			*/
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

		resolvedId := ""
		for line, err = reader.ReadString('\n'); true; line, err = reader.ReadString('\n') {
			if err != nil {
				log.Println("Failed to read following lines")
				return
			}

			log.Println(line)

			if len(line) > 10 && (line[:10] == "Clientid: " || line[:10] == "clientid: ") { //2021-10-05增加，cloudflare 会把http头名改成小写
				log.Println("Found clientid!")
				resolvedId = line[10:30]
				log.Println(resolvedId)
			}

			if line == "\r\n" {
				break
			}
		}

		if len(resolvedId) > 1 {
			fmt.Fprintf(clientConn, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nContent-Type: application/octet-stream\r\nConnection: keep-alive\r\n\r\n")
			/*    fmt.Fprintf(clientConn, "Upgrade: websocket\r\n")
			            fmt.Fprintf(clientConn, "Connection: Upgrade\r\n")
						fmt.Fprintf(clientConn, "Content-Type: application/octet-stream\r\n")
						fmt.Fprintf(clientConn, "Connection: keep-alive\r\n")
						fmt.Fprintf(clientConn, "Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=\r\n")
					//	fmt.Fprintf(clientConn, "Content-Encoding: gzip\r\n")
						fmt.Fprintf(clientConn, "Strict-Transport-Security: max-age=15724800; includeSubDomains\r\n")
						fmt.Fprintf(clientConn, "Transfer-Encoding: chunked\r\n\r\n")
			*/
			//	fmt.Fprintf(clientConn, "Content-Length: 999999\r\n\r\n")
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

	} else if line == "GET /dw HTTP/1.1\r\n" {
		server, err := net.Dial("tcp", v2proxy)
		if err != nil {
			log.Println("error to connect to v2ray:", err)
			return
		}
		str := line
		for line, err = reader.ReadString('\n'); true; line, err = reader.ReadString('\n') {
			if err != nil {
				log.Println("Failed to read following lines", err)
				return
			}

			str += line

			if line == "\r\n" {
				break
			}
		}

		server.Write([]byte(str))
		go io.Copy(server, clientConn)
		io.Copy(clientConn, server)

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
