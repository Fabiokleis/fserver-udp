package main

import (
	w "fserver-udp/server/pkg/worker"
	"log/slog"
	"net"
	"os"
	"sync"
)

const (
	UDP_BIND_ADDRESS = "0.0.0.0:2224"
	MAX_PACKET_SIZE  = 256 // client only send short messages, file name length <= 200
)

type Server struct {
	mu      sync.Mutex
	Workers map[string]*w.Worker
}

func (s *Server) monitor() {
	for {
		s.mu.Lock()
		for addr, worker := range s.Workers {
			if worker.Done {
				delete(s.Workers, addr)
				slog.Info("exiting worker", "address", addr)
			}
		}
		s.mu.Unlock()

	}
}

func (s *Server) sendPacket(addr string, packet []byte) bool {
	s.mu.Lock()
	if w, ok := s.Workers[addr]; ok {
		w.PacketChannel <- packet // send new packet
		s.mu.Unlock()
		return true
	}
	s.mu.Unlock()
	return false
}

func (s *Server) addWorker(worker w.Worker, packet []byte) {
	s.mu.Lock()

	s.Workers[worker.Addr.String()] = &worker

	// start a new worker
	go worker.Execute()
	worker.PacketChannel <- packet

	s.mu.Unlock()
}

func server(socket *net.UDPConn) {

	defer (*socket).Close()

	server := Server{
		Workers: map[string]*w.Worker{},
	}

	go server.monitor()

	packet := make([]byte, MAX_PACKET_SIZE)
	for {
		n, addr, err := (*socket).ReadFrom(packet)
		if err != nil {
			slog.Error("failed to read udp client socket", "address", addr, "error", err)
			continue
		}

		if !server.sendPacket(addr.String(), packet[:n]) {
			server.addWorker(w.Worker{
				Socket:        socket,
				Addr:          addr,
				PacketChannel: make(chan []byte),
				Waiting:       false,
				Done:          false,
			}, packet[:n])
		}
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
