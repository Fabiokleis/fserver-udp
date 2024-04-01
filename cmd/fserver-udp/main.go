package main

import (
	w "fserver-udp/server/pkg/worker"
	"log/slog"
	"net"
	"os"
)

const (
	UDP_BIND_ADDRESS = "0.0.0.0:2224"
	MAX_PACKET_SIZE  = 32
)

func monitor(workers map[string]*w.Worker) {
	for {
		for addr, worker := range workers {
			if (*worker).Done {
				slog.Info("exiting worker", "address", addr)
				delete(workers, addr)
			}
		}
	}
}

func server(socket *net.UDPConn) {

	defer (*socket).Close()

	var workers = map[string]*w.Worker{}

	go monitor(workers)

	packet := make([]byte, MAX_PACKET_SIZE)
	for {
		n, addr, err := (*socket).ReadFrom(packet)
		if err != nil {
			slog.Error("failed to read udp client socket", "address", addr, "error", err)
			continue
		}

		if w, ok := workers[addr.String()]; ok {
			w.PacketChannel <- packet[:n] // send new packet
			continue
		}

		// start a new worker
		worker := &w.Worker{
			Socket:        socket,
			Addr:          addr,
			PacketChannel: make(chan []byte),
			Waiting:       false,
			Done:          false,
		}
		workers[addr.String()] = worker
		go worker.Execute()
		worker.PacketChannel <- packet[:n]
	}
}

func main() {

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.Level(-4)}))

	slog.SetDefault(logger)

	slog.Info("starting udp listener", "address", UDP_BIND_ADDRESS)

	udpAddr, err := net.ResolveUDPAddr("udp", UDP_BIND_ADDRESS)

	if err != nil {
		slog.Error("failed to resolve udp address", "error", err)
		os.Exit(1)
	}
	listener, err := net.ListenUDP("udp", udpAddr)

	if err != nil {
		slog.Error("failed starting udp listener", "error", err)
		os.Exit(1)
	}

	server(listener)
}
