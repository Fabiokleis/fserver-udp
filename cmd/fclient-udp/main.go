package main

import (
	"fmt"
	f "fserver-udp/server/pkg/file"
	msg "fserver-udp/server/pkg/proto"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/codingsince1985/checksum"
	pb "google.golang.org/protobuf/proto"
)

const (
	UDP_SERVER_ADDRESS = "0.0.0.0:2224"
	FILE               = "messages.proto"
	MAX_PACKET_SIZE    = 256
)

type Client struct {
	Socket     *net.Conn         // client connection
	File       f.TokenizableFile // to reconstruct file
	OutputFile string            // transfered file path
}

func (c *Client) readFileChunkPacket(buffer []byte) (*msg.FileChunk, error) {
	fileChunk := msg.FileChunk{}

	if err := pb.Unmarshal(buffer, &fileChunk); err != nil {
		return nil, err
	}

	idx, err := strconv.Atoi(strings.Split(fileChunk.Token, "_")[2])

	if err != nil {
		return nil, err
	}

	c.File.PushToken(idx, true)
	c.File.Size += len(fileChunk.Chunk)
	c.File.Buffer = append(c.File.Buffer, fileChunk.Chunk...)

	return &fileChunk, nil
}

func (c *Client) readConfirmationPacket(buffer []byte) (*msg.Confirmation, error) {
	confirm := &msg.Confirmation{}

	if err := pb.Unmarshal(buffer, confirm); err != nil {
		return nil, err
	}

	return confirm, nil
}

func (c *Client) sendConfirmationPacket(result msg.Result, hash string) {
	confirmation := msg.Confirmation{Result: result, Token: hash}
	cBuffer, _ := pb.Marshal(&confirmation)

	pBuffer := make([]byte, len(cBuffer)+1)
	pBuffer[0] = byte(msg.Verb_CONFIRMATION)
	copy(pBuffer[1:], cBuffer)

	(*c.Socket).Write(pBuffer)
}

func (c *Client) requestMissedPacket() {}

func (c *Client) readPacket(buffer []byte) (*msg.Confirmation, error) {
	switch int(buffer[0]) {
	case int(msg.Verb_RESPONSE):
		fileChunk, err := c.readFileChunkPacket(buffer[1:])
		if err != nil {
			return &msg.Confirmation{Result: msg.Result_PACKET_MISS}, nil
		}
		c.sendConfirmationPacket(msg.Result_OK, fileChunk.Token)
		return &msg.Confirmation{Result: msg.Result_OK}, nil
	case int(msg.Verb_CONFIRMATION):
		confirm, err := c.readConfirmationPacket(buffer[1:])
		if err != nil {
			return nil, err
		}
		return confirm, nil
	default:
		break
	}
	// unidentified format
	return &msg.Confirmation{Result: msg.Result_INVALID_PACKET_FORMAT}, nil
}

func (c *Client) validateCheckSum(hash string) error {

	err := os.WriteFile(c.OutputFile, c.File.Buffer, 0644)
	if err != nil {
		return err
	}
	sha256, _ := checksum.SHA256sum(c.OutputFile)

	if sha256 == hash {
		c.sendConfirmationPacket(msg.Result_VALID_CHECKSUM, "") // server doens't validate checksum
		return nil
	}

	return fmt.Errorf("invalid checksum, file currupted")
}

func requestFilePacket() []byte {
	request := msg.RequestFile{FilePath: FILE}
	rBuffer, _ := pb.Marshal(&request)

	buffer := make([]byte, len(rBuffer)+1)
	buffer[0] = byte(msg.Verb_REQUEST)
	copy(buffer[1:], rBuffer)
	return buffer
}

func main() {
	fmt.Println("starting udp client", UDP_SERVER_ADDRESS)

	conn, err := net.Dial("udp", UDP_SERVER_ADDRESS)
	if err != nil {
		fmt.Println("cannot start udp connection, cause", err)
		os.Exit(1)
	}

	defer conn.Close()

	if _, err := conn.Write(requestFilePacket()); err != nil {
		fmt.Println("cannot write request file message, cause", err)
		os.Exit(1)
	}

	client := Client{
		Socket:     &conn,
		OutputFile: FILE + ".copy",
		File: f.TokenizableFile{
			CheckSum: "",
			Size:     0,
			Tokens:   []*f.Token{},
		},
	}

	packet := make([]byte, MAX_PACKET_SIZE)

	for {
		n, err := conn.Read(packet)
		if err != nil {
			fmt.Println("failed to read udp socket, cause", err)
			continue
		}

		confirm, err := client.readPacket(packet[:n])
		fmt.Println(confirm)

		switch confirm.Result {
		case msg.Result_FILE_NOT_FOUND:
			fmt.Println("could not request file, cause file doesn't exist on server", UDP_SERVER_ADDRESS)
			return
		case msg.Result_PACKET_MISS:
			client.requestMissedPacket()
			break
		case msg.Result_VALID_CHECKSUM:
			if err := client.validateCheckSum(confirm.Token); err != nil {
				fmt.Println(err)
				err := os.Remove(client.OutputFile)
				if err != nil {
					fmt.Println(err)
				}
				return
			}
			fmt.Println("file transfering succeed")
			return
		case msg.Result_OK, msg.Result_INVALID_PACKET_FORMAT:
		default:
			continue
		}
	}
}
