package client

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

type Client struct {
	Socket            *net.Conn         // client connection
	File              f.TokenizableFile // to reconstruct file
	OutputFile        string            // transfered file path
	Transfering       bool              // quit control
	MissPacketChannel chan bool         // check if a packet was missed
	C                 int               // force miss packet
}

func (c *Client) RequestFile(file string) {
	c.Transfering = true
	request := msg.RequestFile{FilePath: file}
	rBuffer, _ := pb.Marshal(&request)

	buffer := make([]byte, len(rBuffer)+1)
	buffer[0] = byte(msg.Verb_REQUEST)
	copy(buffer[1:], rBuffer)

	(*c.Socket).Write(buffer)
}

func (c *Client) sendConfirmationPacket(result msg.Result, hash string) {
	confirmation := msg.Confirmation{Result: result, Token: hash}
	cBuffer, _ := pb.Marshal(&confirmation)

	pBuffer := make([]byte, len(cBuffer)+1)
	pBuffer[0] = byte(msg.Verb_CONFIRMATION)
	copy(pBuffer[1:], cBuffer)

	(*c.Socket).Write(pBuffer)
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

func (c *Client) validateCheckSum(hash string) error {

	err := os.WriteFile(c.OutputFile, c.File.Buffer, 0644)
	if err != nil {
		return err
	}
	sha256, _ := checksum.SHA256sum(c.OutputFile)

	if sha256 == hash {
		c.File.CheckSum = sha256
		c.sendConfirmationPacket(msg.Result_VALID_CHECKSUM, "") // server doens't validate checksum
		return nil
	}

	return fmt.Errorf("invalid checksum, file currupted")
}

func (c *Client) verifyConfirmation(confirm *msg.Confirmation) {
	switch confirm.Result {
	case msg.Result_FILE_NOT_FOUND:
		fmt.Println("could not request file, cause file doesn't exist on server", (*c.Socket).RemoteAddr().String())
		return
	case msg.Result_VALID_CHECKSUM:
		if err := c.validateCheckSum(confirm.Token); err != nil {
			fmt.Println(err)
			err := os.Remove(c.OutputFile)
			if err != nil {
				fmt.Println(err)
			}
			break
		}
		fmt.Println("sha256 checksum", c.File.CheckSum)
		fmt.Println("file transfering succeed")
		c.Transfering = false // stop reading packets
		break
	case msg.Result_OK, msg.Result_INVALID_PACKET_FORMAT:
	default:
		break
	}
}

func (c *Client) sendMissPacket() {
	request := msg.Confirmation{Result: msg.Result_PACKET_MISS}
	rBuffer, _ := pb.Marshal(&request)

	buffer := make([]byte, len(rBuffer)+1)
	buffer[0] = byte(msg.Verb_CONFIRMATION)
	copy(buffer[1:], rBuffer)

	(*c.Socket).Write(buffer)
}

func (c *Client) KeepCheckingServer() {
	for c.Transfering {
		select {
		case check := <-c.MissPacketChannel:
			if check {
				c.sendMissPacket()
			}
			break
		default:
			break
		}
	}
}

func (c *Client) ReadPacket(buffer []byte) {

	switch int(buffer[0]) {
	case int(msg.Verb_RESPONSE):
		fileChunk, err := c.readFileChunkPacket(buffer[1:])
		if err != nil {
			fmt.Println("failed file chunk", err)
			break
		}
		//time.Sleep(time.Duration(1) * time.Second)
		c.sendConfirmationPacket(msg.Result_OK, fileChunk.Token)
	case int(msg.Verb_CONFIRMATION):
		confirm := &msg.Confirmation{}
		if err := pb.Unmarshal(buffer[1:], confirm); err != nil {
			fmt.Println("failed confirmation", err)
			break
		}
		c.verifyConfirmation(confirm)
		break
	default:
		break
	}
}
