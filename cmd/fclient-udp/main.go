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
	MAX_PACKET_SIZE    = 1100 // 1024 file chunk + proto encoding + safety
	MAX_TIMEOUT        = 3
)

/*
   easy fixed size file
   head -c 10M </dev/urandom > myfile
*/

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

	if len(*file) > 200 {
		fmt.Println("file name too long")
		os.Exit(1)
	}

	missed := 0
	if *missPacket == true {
		missed = rand.Intn(5) + 1 // random packet to miss
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
		Socket:        &conn,
		Transfering:   false,
		PacketChannel: make(chan []byte),
		File: f.TokenizableFile{
			CheckSum: "",
			Size:     0,
			Tokens:   []*f.Token{},
		},
	}

	packet := make([]byte, MAX_PACKET_SIZE)

	client.RequestFile(*file)

	go client.KeepCheckingServer(*missPacket, 1)

	for client.Transfering {
		conn.SetReadDeadline(time.Now().Add(time.Second * MAX_TIMEOUT))
		n, err := conn.Read(packet)
		if err != nil {
			break
		}

		//fmt.Println("packet: ", packet[:n])

		client.PacketChannel <- packet[:n]
		//fmt.Println(client.Transfering)
	}
}
