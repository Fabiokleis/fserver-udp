package main

import (
	"fmt"
	msg "fserver-udp/pkg/proto"
	"net"

	pb "google.golang.org/protobuf/proto"
)

const (
	UDP_BIND_ADDRESS = "0.0.0.0:2224"
	BUFF_SIZE        = 32
)

func handleClient(socket net.Conn) {

	defer socket.Close()

	buffer := make([]byte, BUFF_SIZE)

	n, err := socket.Read(buffer)

	fmt.Println("read %d bytes", n)

	if err != nil {
		fmt.Printf("failed to read udp client socket %s, cause %s\n", socket.RemoteAddr(), err)
		return
	}

	request := msg.Message{}

	if err := pb.Unmarshal(buffer[:], &request); err != nil {
		fmt.Printf("failed to deacode buffer as request %s, cause %s", buffer, err)
	}
}

func main() {
	fmt.Println("start udp listener ", UDP_BIND_ADDRESS)

	listener, err := net.Listen("udp", UDP_BIND_ADDRESS)

	if err != nil {
		fmt.Printf("failed to start udp listener %s, cause %s\n", UDP_BIND_ADDRESS, err)
	}

	for {
		client, err := listener.Accept()

		if err != nil {
			fmt.Printf("failed to handle udp client, cause %s\n", err)
		}

		go handleClient(client)
	}
}
