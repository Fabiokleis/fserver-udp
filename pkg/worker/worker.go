package worker

import (
	"bytes"
	"fmt"
	"net"
	"os"

	msg "fserver-udp/server/pkg/proto"

	"github.com/codingsince1985/checksum"
	pb "google.golang.org/protobuf/proto"
)

const (
	BUFF_SIZE  = 32
	CHUNK_SIZE = 255
)

type Worker struct {
	Socket        *net.UDPConn
	Addr          net.Addr
	PacketChannel chan []byte
	File          msg.TokenizableFile
	Done          bool // to stop executing
}

func (w *Worker) sendConfirmationPacket(result msg.Result, token string) {
	confirmation := msg.Confirmation{Result: result, Token: token}
	cBuffer, _ := pb.Marshal(&confirmation)

	pBuffer := make([]byte, CHUNK_SIZE+1)
	pBuffer[0] = byte(msg.Verb_CONFIRMATION)
	copy(pBuffer[1:], cBuffer)

	(*w.Socket).WriteTo(cBuffer, w.Addr)
}

func (w *Worker) confirmPacket(hash string) error {
	token, ok := w.File.Tokens[hash]
	if !ok {
		return fmt.Errorf("token not found")
	}

	if token.Received {
		return fmt.Errorf("token already confirmed")
	}

	w.File.Tokens[hash] = &msg.Token{Index: token.Index, Received: true}

	return nil
}

func (w *Worker) sendFilePacket(hash string) {
	token := w.File.Tokens[hash]
	block := token.Index + CHUNK_SIZE

	if block > w.File.Size {
		block = w.File.Size
	}

	chunkBuffer, _ := pb.Marshal(
		&msg.FileChunk{
			Chunk:    w.File.Buffer[token.Index:block],
			Token:    hash,
			CheckSum: w.File.CheckSum,
		},
	)
	pBuffer := make([]byte, CHUNK_SIZE+1)
	pBuffer[0] = byte(msg.Verb_RESPONSE)
	copy(pBuffer[1:], chunkBuffer)

	(*w.Socket).WriteTo(pBuffer, w.Addr)
}

func (w *Worker) sendFile() {
	for hash := range w.File.Tokens {
		w.sendFilePacket(hash)
	}
}

func (w *Worker) tokenizeFile(buffer []byte) {
	size := len(buffer)
	tokens := make(map[string]*msg.Token)
	for i := 0; i < size; i += CHUNK_SIZE {
		hash := fmt.Sprintf("token_idx_%d", i)
		tokens[hash] = &msg.Token{
			Index:    uint32(i),
			Received: false,
		}
	}

	w.File = msg.TokenizableFile{
		Buffer: buffer,
		Size:   uint32(size),
		Tokens: tokens,
	}
}

func (w *Worker) readFile(buffer []byte) ([]byte, msg.Result, error) {
	request := msg.RequestFile{}
	if err := pb.Unmarshal(bytes.Trim(buffer, "\x00"), &request); err != nil {
		return nil, msg.Result_INVALID_PACKET_FORMAT, err
	}
	fBuffer, err := os.ReadFile(request.FilePath)
	if err != nil {
		return nil, msg.Result_FILE_NOT_FOUND, err
	}

	sha256, err := checksum.SHA256sum(request.FilePath)
	if err != nil {
		return nil, msg.Result_ERROR_CHECK_SUM, err
	}
	fmt.Printf("sha256 checksum %v -> %s\n", request.FilePath, sha256)
	w.File.CheckSum = sha256

	return fBuffer, msg.Result_OK, nil
}

func (w *Worker) Execute() {
	defer func() {
		close(w.PacketChannel)
	}()

	fmt.Println("executing new worker....")

	for {
		select {
		case buffer := <-w.PacketChannel:
			switch int(buffer[0]) {
			case int(msg.Verb_REQUEST):
				fileBuffer, result, err := w.readFile(buffer[1:]) // skip identification byte
				switch result {
				case msg.Result_INVALID_PACKET_FORMAT:
					fmt.Printf("failed to decode buffer as protobuf message, cause %s\n", err)
					w.sendConfirmationPacket(result, "")
					return // exit worker

				case msg.Result_FILE_NOT_FOUND:
					fmt.Printf("failed to read file, cause %s\n", err)
					w.sendConfirmationPacket(result, "")
					return // exit worker

				case msg.Result_ERROR_CHECK_SUM:
					fmt.Printf("failed to generate sha256, cause %s\n", err)
					w.sendConfirmationPacket(result, "")
					return // exit worker

				case msg.Result_OK:
					fmt.Println("sending all file chunks packets....")
					w.tokenizeFile(fileBuffer)
					w.sendFile() // send all file chunks
					fmt.Println("file sent")
					break
				default:
					return // wtf
				}

			case int(msg.Verb_CONFIRMATION):
				confirm := msg.Confirmation{}
				if err := pb.Unmarshal(buffer[1:BUFF_SIZE], &confirm); err != nil {
					fmt.Printf("failed to decode buffer as protobuf message, cause %s\n", err)
					w.sendConfirmationPacket(msg.Result_INVALID_PACKET_FORMAT, "")
					return // exit worker
				}
				switch confirm.Result {
				case msg.Result_OK:
					fmt.Println("received packet, token hash ", confirm.Token)
					if err := w.confirmPacket(confirm.Token); err != nil {
						fmt.Printf("cannot confirm packet, failed to find token hash %s, cause %s\n", confirm.Token, err)
						w.sendConfirmationPacket(msg.Result_INVALID_TOKEN, confirm.Token)
					}
					break
				case msg.Result_PACKET_MISS:
					fmt.Println("resending packet, token hash ", confirm.Token)
					w.sendFilePacket(confirm.Token) // resend missed packet
					break
				case msg.Result_VALID_CHECKSUM:
					fmt.Println("checksum validation worked")
					w.Done = true
					return
				default:
					return // wtf
				}
			}

		default:
			//fmt.Println("waiting packet.....")
		}
	}
}
