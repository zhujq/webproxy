package main

import (
	"bufio"
//	"fmt"
	"io"
	"log"
	"math/rand"
	"time"
	"net"
	"crypto/tls"
	"encoding/json"
	"os"
	"strings"
//	"sync"
//	"crypto/x509"
//	"io/ioutil"
//  "github.com/robfig/cron"

)

//const proxyDomain = "ub-proxy-service-zhujq.cloud.okteto.net"
//const port = "9999"

var letters = []rune("abcdefghijklmnopqrstuvwyz1234567890")

type Serv_port struct {
    Port  string `json:"port"`
    Webserv string `json:"address"`
}

var webserv_port map[string]string

func randSeq(n int) string {
	b := make([]rune, n)
	rand.Seed(time.Now().Unix())
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func makedefaultcf(){
	webserv_port["9999"] = "ub-proxy-service-zhujq.cloud.okteto.net"	
	webserv_port["9997"] = "zhujq-ssh.run.goorm.io"	
}

func handleConnection(clientConn net.Conn,webServer string) {
	defer clientConn.Close()

//	conf := &tls.Config{
//		InsecureSkipVerify: true,
//	}
/*	pool := x509.NewCertPool()
	caCertPath := "cloud-okteto-net-chain.pem"

	caCrt, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		log.Println("ReadFile err:", err)
		return
	}
	pool.AppendCertsFromPEM(caCrt)
	conf := &tls.Config{RootCAs: pool}
*/	conf := &tls.Config{InsecureSkipVerify: true}
		
	proxyDomain := webServer
	
	var err error
	var serverListen,serverSend net.Conn

	if strings.Contains(proxyDomain,":") == false {   //地址信息不含端口号
		proxyDomain += ":"
	}
	if strings.HasSuffix(proxyDomain,":") {   //默认443端口
		proxyDomain += "443"
	}

	if  strings.HasSuffix(proxyDomain,":443"){
		serverListen, err = tls.Dial("tcp", proxyDomain,conf)
	}else{
		serverListen, err = net.Dial("tcp", proxyDomain)
	}

	if err != nil {
		log.Println(proxyDomain+":Failed to connect to listen proxy server!",err)
		return
	}

	log.Println(proxyDomain+":Success dial to Server_listen")

/*	err = serverListen.Handshake() 
	if err != nil {
		log.Println(proxyDomain+":Failed to ssl handshake!",err)
		return
	}
*/
	if  strings.HasSuffix(proxyDomain,":443"){
		serverSend, err = tls.Dial("tcp", proxyDomain,conf)
	}else{
		serverSend, err = net.Dial("tcp", proxyDomain)
	}

	if err != nil {
		log.Println(proxyDomain+":Failed to connect to send proxy server! ",err)
		return
	}

	log.Println(proxyDomain+":Success dial to Server_send")

	defer serverListen.Close()
	defer serverSend.Close()

	clientId := randSeq(20)

	wait := make(chan bool)

	go func() {
		log.Println(proxyDomain+":starting to get websocket upgrade")

		_, err =serverListen.Write([]byte("GET /listen HTTP/1.1\r\n"+"Host: "+webServer+"\r\n"+"Accept: */*\r\n"+"Upgrade: websocket\r\n"+"Connection: Upgrade\r\n"+"Clientid: "+clientId+"\r\n"+"Connection: keep-alive\r\n"+ "Sec-WebSocket-Version: 13\r\n"+ "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +"\r\n"))
		if err != nil {
			log.Println(proxyDomain+":Error write to serverlisten", err)
		}
	
		buf := bufio.NewReader(serverListen)			
		success := false

		timer2 := time.NewTimer(time.Second*30)    
		go func() {        //等触发时的信号
			for {
				if success == true{
			//		serverListen.Write([]byte{0x9})    // 0x9=ping (websocket keepalive frame)
					serverListen.Write([]byte(" ")) 
					log.Println(proxyDomain+":serverListen Keepalive...")
				}
				timer2 = time.NewTimer(time.Second*30)
        		<-timer2.C
			}
    	}() 

		for line, err := buf.ReadString('\n'); true; line, err = buf.ReadString('\n') {
			log.Println(line)
			if err != nil {
				log.Println("error:", err)
				log.Println(proxyDomain+":Failed to read following lines")
				return
			}

			if line == "HTTP/1.1 101 Switching Protocols\r\n" {
				success = true	
			}

			if success && line == "\r\n" {
				break
			}
		}

		if success {
			_, err = io.Copy(clientConn, buf)

			if err != nil {
				log.Println("error:", err)
				log.Println("Error copying server to client stream", err)
			}
		} else {
			log.Println("Failed to bind listen connection!")
		}
		timer2.Stop()
		wait <- true

	}()

	go func() {
		log.Println(proxyDomain+":starting to post websocket upgrade")

		_, err =serverSend.Write([]byte("GET /transmit HTTP/1.1\r\n"+"Host: "+webServer+"\r\n"+"Accept: */*\r\n"+"Upgrade: websocket\r\n"+"Connection: Upgrade\r\n"+"Clientid: "+clientId+"\r\n"+"Connection: keep-alive\r\n"+ "Sec-WebSocket-Version: 13\r\n"+ "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +"\r\n"))
		if err != nil {
			log.Println(proxyDomain+":Error write to serversend", err)
		}

		buf := bufio.NewReader(serverSend)
		success := false
		
		timer2 := time.NewTimer(time.Second*30)    
		go func() {        //等触发时的信号
			for {
				if success == true{
			//		serverSend.Write([]byte{0x9})
					serverListen.Write([]byte(" ")) 
					log.Println(proxyDomain+":serverSend Keepalive...")
				}
				timer2 = time.NewTimer(time.Second*30)
        		<-timer2.C
			}
    	}() 
		
		for line, err := buf.ReadString('\n'); true; line, err = buf.ReadString('\n') {
			log.Println(line)
			if err != nil {
				log.Println("error:", err)
				log.Println(proxyDomain+":Failed to read following lines")
				return
			}

			if line == "HTTP/1.1 101 Switching Protocols\r\n" {
				success = true
			}

			if success && line == "\r\n" {
				break
			}
		}

		if success {
    
			_, err = io.Copy(serverSend, clientConn)
			if err != nil {
				log.Println(proxyDomain+":Error copying client to server stream", err)
			}
		}else {
			log.Println(proxyDomain+":Failed to bind send connection!")
		}
		timer2.Stop()
		log.Println(proxyDomain+":session is over")
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
