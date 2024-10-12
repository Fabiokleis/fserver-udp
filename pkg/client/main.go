package client

import (
	"fmt"
	f "fserver-udp/server/pkg/file"
	msg "fserver-udp/server/pkg/proto"
	"net"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/codingsince1985/checksum"
	pb "google.golang.org/protobuf/proto"
)

const (
	RESEND_MAX_TIME = 2
)

type Client struct {
	Socket        *net.Conn         // client connection
	File          f.TokenizableFile // to reconstruct file
	Transfering   bool              // quit control
	PacketChannel chan []byte       // check if a packet was read
	C             int
}

func (c *Client) RequestFile(file string) {
	c.Transfering = true

	request := msg.RequestFile{FilePath: path.Clean(file)}
	rBuffer, _ := pb.Marshal(&request)

	buffer := make([]byte, len(rBuffer)+1)
	buffer[0] = byte(msg.Verb_REQUEST)
	copy(buffer[1:], rBuffer)

	_, filePath := path.Split(file)
	c.File.CreateFile(filePath + ".copy")

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

	idx, err := strconv.ParseUint(strings.Split(fileChunk.Token, "_")[2], 10, 32)

	if err != nil {
		return nil, err
	}

	if c.File.FindToken(fileChunk.Token) != nil {
		c.sendConfirmationPacket(msg.Result_OK, fileChunk.Token)
		return &fileChunk, fmt.Errorf("already wrote file chunk, skipping")
	}
	c.File.PushToken(idx, true)
	c.File.WriteChunk(fileChunk.Chunk)

	return &fileChunk, nil
}

func (c *Client) validateCheckSum(hash string) error {

	sha256, _ := checksum.SHA256sum(c.File.Path())

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
			if err := c.File.DeleteFile(); err != nil {
				fmt.Println(err)
			}
			c.Transfering = false // stop reading packets
			return
		}
		c.Transfering = false // stop reading packets
		fmt.Println("sha256 checksum", c.File.CheckSum)
		fmt.Println("file transfering succeed")

		break
	case msg.Result_OK, msg.Result_INVALID_PACKET_FORMAT:
		token := c.File.LastReceivedToken()
		if token != nil {
			c.sendConfirmationPacket(
				msg.Result_OK,
				f.TokenHash(token.Index),
			)
			fmt.Println("re-sending confirmation token")
		}
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

func (c *Client) KeepCheckingServer(miss bool, missed int) {

	defer func() {
		close(c.PacketChannel)
	}()

	timer := time.NewTimer(time.Duration(RESEND_MAX_TIME) * time.Second)
	for c.Transfering {
		select {
		case <-timer.C:
			//fmt.Println("calling")
			c.sendMissPacket()
			timer.Reset(time.Duration(RESEND_MAX_TIME) * time.Second)
			break
		case packet := <-c.PacketChannel:

			//fmt.Println("packet: ", packet)
			timer.Reset(time.Duration(RESEND_MAX_TIME) * time.Second)

			if miss {
				if c.C < missed {
					c.C++
				} else if c.C == missed {
					c.C = missed + 1
					break // miss packet
				}
			}

			c.readPacket(packet)

			break
		default:
			break
		}
	}
}

func (c *Client) readPacket(buffer []byte) {

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
