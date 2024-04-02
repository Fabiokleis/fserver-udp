package main

import (
	"fmt"
	f "fserver-udp/server/pkg/file"
	"net"
	"os"
	"time"

	"flag"
	c "fserver-udp/server/pkg/client"
	"math/rand"
)

const (
	UDP_SERVER_ADDRESS = "0.0.0.0:2224"
	MAX_PACKET_SIZE    = 256
	RESPONSE_MAX_TIME  = 2
)

func main() {

	bind := flag.String("bind", "", "file server host:port")
	file := flag.String("file", "", "file absolute path")
	missPacket := flag.Bool("miss", false, "force to miss one upd packet")

	flag.Parse()

	if *bind == "" {
		fmt.Println("missing `--bind` flag")
		os.Exit(1)
	}

	if *file == "" {
		fmt.Println("missing `--file` flag")
		os.Exit(1)
	}

	missed := 0
	if *missPacket == true {
		missed = rand.Intn(3) + 1
		fmt.Println("client will miss packet", missed)
	}

	fmt.Println("starting udp client", *bind)

	conn, err := net.Dial("udp", *bind)
	if err != nil {
		fmt.Println("cannot start udp connection, cause", err)
		os.Exit(1)
	}

	defer conn.Close()

	client := c.Client{
		Socket:            &conn,
		OutputFile:        *file + ".copy",
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

	client.RequestFile(*file)
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
