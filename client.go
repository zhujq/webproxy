package main

import (
	rand2 "crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
	"golang.org/x/net/websocket"
)


var letters = []rune("abcdefghijklmnopqrstuvwyz1234567890")
var keyGUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

type Serv_port struct {
    Port  string `json:"port"`
    Webserv string `json:"address"`
}

var webserv_port map[string]string

func secWebSocketKey() (string, error) {
	rr := rand2.Reader
	b := make([]byte, 16)
	_, err := io.ReadFull(rr, b)
	if err != nil {
		return "", fmt.Errorf("failed to read random data from rand.Reader: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func randSeq(n int) string {
	b := make([]rune, n)
	rand.Seed(time.Now().Unix())
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func makedefaultcf(){
	webserv_port["9991"] = "web.zhujq.ml"
}

func handleConnection(clientConn net.Conn,webServer string) {
	defer clientConn.Close()
	var slconfig, ssconfig websocket.Config
	slconfig.TlsConfig = &tls.Config{InsecureSkipVerify: true}
	ssconfig.TlsConfig = &tls.Config{InsecureSkipVerify: true}
	var serverListen, serverSend string
	proxyDomain := webServer

	if strings.Contains(proxyDomain,":") == false {   //地址信息不含端口号
		proxyDomain += ":"
	}
	if strings.HasSuffix(proxyDomain,":") {   //默认443端口
		proxyDomain += "443"
	}

	if strings.HasSuffix(proxyDomain, ":443") {
		serverListen = ("wss://" + proxyDomain + "/listen")
		serverSend = ("wss://" + proxyDomain + "/transmit")
	} else {
		serverListen = ("ws://" + proxyDomain + "/listen")
		serverSend = ("ws://" + proxyDomain + "/transmit")
	}

	slurl, _ := url.Parse(serverListen)
	ori, _ := url.Parse("https://" + proxyDomain)
	ssurl, _ := url.Parse(serverSend)
	slconfig.Location = slurl
	slconfig.Origin = ori
	slconfig.Version = 13
	ssconfig.Location = ssurl
	ssconfig.Origin = ori
	ssconfig.Version = 13

	clientId := randSeq(20)
	wait := make(chan bool)

	go func() {
		log.Println(webServer + ":Starting  listen-websocket upgrade")
		conn, err := websocket.DialConfig(&slconfig)
		if err != nil {
			log.Println(webServer+":Error websocket-dail to serverListen", err)
			return
		}
		defer conn.Close()
		log.Println(webServer + ":Succed to listen-websocket upgrade")

		_, err = conn.Write([]byte("Clientid: " + clientId + "\r\n"))
		if err != nil {
			log.Println(webServer+":Error write to serverlisten", err)
		}
		log.Println(webServer + ":Succed to send clientid by listen-websocket,entering io copy mode....")

		_, err = io.Copy(clientConn, conn)
		if err != nil {
			log.Println(webServer+":Error copying data from websocket-server to client stream,error is ", err)
		}

		wait <- true

	}()

	go func() {
		log.Println(webServer + ":starting send-websocket upgrade")
		conn, err := websocket.DialConfig(&ssconfig)
		if err != nil {
			log.Println(webServer+":Error websocket-dail to serverSend", err)
			return
		}
		defer conn.Close()
		log.Println(webServer + ":succed to send-websocket upgrade")
		_, err = conn.Write([]byte("Clientid: " + clientId + "\r\n"))
		if err != nil {
			log.Println(webServer+":Error write to serverSend", err)
			return
		}
		log.Println(webServer + ":succed to send clientid by send-websocket,entering io copy mode....")

		_, err = io.Copy(conn, clientConn)
		if err != nil {
			log.Println(webServer+":Error copying data from clientConn to websocket-server,error is ", err)
		}
		log.Println(webServer + ":session is over")
		wait <- true
	}()

	<-wait
}

func main() {
	log.Println("This program is designed by zhujq for ssh over http/https,starting...")

	webserv_port = make(map[string]string)

	file, err := os.Open("./cf.json")
	if err != nil {	  //无法读取配置文件时用默认配置
		log.Println("error to read config file,use default config")
		makedefaultcf()
	}else {
		buf := make([]byte, 1024)
		len, _ := file.Read(buf)

        if len == 0 {
			log.Println("config file is empty,use default config")
            makedefaultcf()
        }else{
			b := string(buf)
			var Servlist []Serv_port
			err = json.Unmarshal([]byte(b[:len]), &Servlist)
        	if err != nil {
				log.Println("decode json file error,use default config")
            	makedefaultcf()
			}else{
				for _ ,a:=range Servlist {
					webserv_port[a.Port] = a.Webserv
				}
			}
		}
	}

	defer file.Close()

	done := make(chan bool)

	for port := range webserv_port {

		go func(bport string) {

			ln, err := net.Listen("tcp", ":"+bport)
			if err != nil {
				log.Println("Error listening!", err)
				return
			}

			log.Println("Listening at "+bport+" for ssh over http(s)://"+webserv_port[bport])

			for true {
				conn, err := ln.Accept()
				if err != nil {
					log.Println("Error accepting connection", err)
					continue
				}

			go handleConnection(conn,webserv_port[bport])

			}

			done <- true

		}(port)

	}

	<-done

}
