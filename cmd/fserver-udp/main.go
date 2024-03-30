package main

import (
	"fmt"
	w "fserver-udp/server/pkg/worker"
	"net"
	"os"
)

const (
	UDP_BIND_ADDRESS = "0.0.0.0:2224"
	BUFF_SIZE        = 32
)

func monitor(workers map[string]*w.Worker) {
	for addr, worker := range workers {
		if (*worker).Done {
			fmt.Printf("stop worker for %s", addr)
			delete(workers, addr)
		}
	}
}

func server(socket *net.UDPConn) {

	defer (*socket).Close()

	var workers = map[string]*w.Worker{}

	go monitor(workers)

	packet := make([]byte, BUFF_SIZE)
	for {
		_, addr, err := (*socket).ReadFrom(packet)
		if err != nil {
			fmt.Printf("failed to read udp client socket %s, cause %s\n", addr, err)
			continue
		}

		fmt.Println("address", addr)
		if w, ok := workers[addr.String()]; ok {
			w.PacketChannel <- packet // send new packet
			continue
		}

		// start a new worker
		worker := &w.Worker{Socket: socket, Addr: addr, PacketChannel: make(chan []byte), Done: false}
		workers[addr.String()] = worker
		go worker.Execute()
		worker.PacketChannel <- packet
	}
}

func main() {
	fmt.Println("starting udp listener", UDP_BIND_ADDRESS)

	udpAddr, err := net.ResolveUDPAddr("udp", UDP_BIND_ADDRESS)

	if err != nil {
		fmt.Printf("failed to resolve udp address, cause %s\n", err)
		os.Exit(1)
	}
	listener, err := net.ListenUDP("udp", udpAddr)

	if err != nil {
		fmt.Printf("failed starting udp listener, cause %s\n", err)
		os.Exit(1)
	}
	server(listener)
}
