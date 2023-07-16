package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

const port = "80"
const target = "127.0.0.1:22"
const v2proxy = "127.0.0.1:8080"

type client struct {
	listenChannel        chan bool       // Channel that the client is listening on
	transmitChannel      chan bool       // Channel that the client is writing to
	listener             *websocket.Conn // The thing to write to
	listenerConnected    bool
	transmitter          *websocket.Conn // The thing to listen from
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

func lsHandler(w *websocket.Conn) {
	resolvedId := ""
	for {
		var message string
		websocket.Message.Receive(w, &message)
		line := string(message)
		if len(line) > 10 && (line[:10] == "Clientid: " || line[:10] == "clientid: ") {
			log.Println("Found clientid!")
			resolvedId = line[10:30]
			log.Println(resolvedId)
			break
		}
	}
	wait := make(chan bool)

	if _, ok := connectedClients[resolvedId]; !ok {
		connectedClients[resolvedId] = client{}
	}

	currentClient := connectedClients[resolvedId]

	currentClient.listener = w
	currentClient.listenChannel = wait
	currentClient.listenerConnected = true

	connectedClients[resolvedId] = currentClient

	log.Println("Attempting to bind listener")

	go bindServer(resolvedId)

	<-wait

}

func tsHandler(w *websocket.Conn) {
	resolvedId := ""
	for {
		var message string
		websocket.Message.Receive(w, &message)
		line := string(message) //读取客户端发生的clientid
		if len(line) > 10 && (line[:10] == "Clientid: " || line[:10] == "clientid: ") {
			log.Println("Found clientid!")
			resolvedId = line[10:30]
			log.Println(resolvedId)
			break
		}
	}

	wait := make(chan bool)

	if _, ok := connectedClients[resolvedId]; !ok {
		connectedClients[resolvedId] = client{}
	}

	currentClient := connectedClients[resolvedId]

	currentClient.transmitter = w
	currentClient.transmitChannel = wait
	currentClient.transmitterConnected = true

	connectedClients[resolvedId] = currentClient

	log.Println("Attempting to bind transmission")

	go bindServer(resolvedId)

	<-wait

}

func defHandler(w *websocket.Conn) {
	var err error
	for {
		var reply string
		if err = websocket.Message.Receive(w, &reply); err != nil {
			log.Println("接受消息失败", err)
			break
		}
		msg := time.Now().String() + reply
		//log.Println(msg)
		if err = websocket.Message.Send(w, msg); err != nil {
			log.Println("发送消息失败")
			break
		}
	}
}

func rayHandler(w http.ResponseWriter, r *http.Request) {
	str := "GET /dw HTTP/1.1\r\n"
	str += ("Host: " + r.Host + "\r\n")
	for k, v := range r.Header {
		str += (k + ": " + strings.Join(v, ",") + "\r\n")
	}
	str += "\r\n"
	log.Println(str)

	//log.Println("Getting ray access request...")
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Println("Hijacker error")
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		log.Println("Hijacker Conn error")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//log.Println("hj.hijacker is ok")
	defer clientConn.Close()

	server, err := net.Dial("tcp", v2proxy)
	if err != nil {
		log.Println("error to connect to v2ray:", err)
		return
	}

	server.Write([]byte(str))
	go io.Copy(server, clientConn)
	io.Copy(clientConn, server)

}

func main() {
	log.Println("Listening...")
	http.Handle("/listen", websocket.Handler(lsHandler))
	http.Handle("/transmit", websocket.Handler(tsHandler))
	http.HandleFunc("/dw", rayHandler)
	http.Handle("/", websocket.Handler(defHandler))
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
