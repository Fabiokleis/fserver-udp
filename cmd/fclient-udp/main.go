package main

import (
	"fmt"
	f "fserver-udp/server/pkg/file"
	"net"
	"os"
	"time"

	c "fserver-udp/server/pkg/client"
)

const (
	UDP_SERVER_ADDRESS = "0.0.0.0:2224"
	FILE               = "messages.proto"
	MAX_PACKET_SIZE    = 256
	RESPONSE_MAX_TIME  = 2
)

func main() {

	fmt.Println("starting udp client", UDP_SERVER_ADDRESS)

	conn, err := net.Dial("udp", UDP_SERVER_ADDRESS)
	if err != nil {
		fmt.Println("cannot start udp connection, cause", err)
		os.Exit(1)
	}

	defer conn.Close()

	client := c.Client{
		Socket:            &conn,
		OutputFile:        FILE + ".copy",
		Transfering:       false,
		MissPacketChannel: make(chan bool),
		File: f.TokenizableFile{
			CheckSum: "",
			Size:     0,
			Tokens:   []*f.Token{},
		},
	}

	timer := time.NewTimer(time.Duration(RESPONSE_MAX_TIME) * time.Second)
	packet := make([]byte, MAX_PACKET_SIZE)

	client.RequestFile(FILE)
	go client.KeepCheckingServer()

	for client.Transfering {
		n, err := conn.Read(packet)
		if err != nil {
			fmt.Println("failed to read udp socket, cause", err)
			break
		}
		//fmt.Println("packet: ", packet[:n])

		select {
		case <-timer.C: // after 2s without receiving packets, request missing packet
			client.MissPacketChannel <- true
			timer.Reset(time.Duration(RESPONSE_MAX_TIME) * time.Second)
			break
		default:
			break
		}

		client.ReadPacket(packet[:n])
	}
}
