package worker

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	f "fserver-udp/server/pkg/file"
	msg "fserver-udp/server/pkg/proto"

	"github.com/codingsince1985/checksum"
	pb "google.golang.org/protobuf/proto"
)

const (
	TIMEOUT    = 5   // secs
	CHUNK_SIZE = 128 // file chunk bytes
)

type Worker struct {
	Socket        *net.UDPConn      // server socket
	Addr          net.Addr          // worker client addr
	PacketChannel chan []byte       // server packet message
	File          f.TokenizableFile // Tokenized file
	Waiting       bool              // control confirmation packets
	Done          bool              // to stop executing
}

func (w *Worker) sendConfirmationPacket(result msg.Result, token string) {
	confirmation := msg.Confirmation{Result: result, Token: token}
	cBuffer, _ := pb.Marshal(&confirmation)

	pBuffer := make([]byte, len(cBuffer)+1)
	pBuffer[0] = byte(msg.Verb_CONFIRMATION)
	copy(pBuffer[1:], cBuffer)

	(*w.Socket).WriteTo(pBuffer, w.Addr)
}

func (w *Worker) confirmPacket(hash string) error {
	token := w.File.FindToken(hash)
	if token == nil {
		return fmt.Errorf("token not found")
	}

	if token.Received {
		return fmt.Errorf("token already confirmed")
	}
	token.Received = true

	return nil
}

func (w *Worker) sendFilePacket(token *f.Token) {

	block := token.Index + CHUNK_SIZE

	if block > w.File.Size {
		block = w.File.Size
	}

	chunkBuffer, _ := pb.Marshal(
		&msg.FileChunk{
			Chunk: w.File.Buffer[token.Index:block],
			Token: f.TokenHash(token.Index),
		},
	)

	pBuffer := make([]byte, len(chunkBuffer)+1) // < 256
	pBuffer[0] = byte(msg.Verb_RESPONSE)
	copy(pBuffer[1:], chunkBuffer)

	(*w.Socket).WriteTo(pBuffer, w.Addr)
}

func (w *Worker) sendFile() {

	cursor := 0 // send packets in order
	for !w.Done {
		if !w.Waiting {
			w.sendFilePacket(w.File.Tokens[cursor])
			w.Waiting = true
			cursor++
		}

		// check if file is already sent
		if cursor == len(w.File.Tokens) {
			w.sendConfirmationPacket(msg.Result_VALID_CHECKSUM, w.File.CheckSum)
			break
		}
	}
}

func (w *Worker) resendLastPacket(hash string) {
	if token := w.File.FindNotReceivedToken(); token != nil {
		w.sendFilePacket(token)
		slog.Info("re-sending packet", "address", w.Addr.String(), "token", f.TokenHash(token.Index))
		return
	}

	// if all tokens was received, only possible missed packet was the checksum confirmation
	w.sendConfirmationPacket(msg.Result_VALID_CHECKSUM, w.File.CheckSum)
	slog.Info("re-sending packet", "address", w.Addr.String(), "token", w.File.CheckSum)
}

func (w *Worker) tokenizeFile(buffer []byte) {
	size := len(buffer)

	w.File.Buffer = buffer
	w.File.Size = size
	w.File.Tokens = []*f.Token{}

	for i := 0; i < size; i += CHUNK_SIZE {
		w.File.PushToken(i, false)
	}
}

func (w *Worker) readFile(buffer []byte) ([]byte, msg.Result, error) {
	request := msg.RequestFile{}
	if err := pb.Unmarshal(buffer, &request); err != nil {
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
	slog.Info("generated file checksum", "address", w.Addr.String(), "file", request.FilePath, "sha256sum", sha256)
	w.File.CheckSum = sha256

	return fBuffer, msg.Result_OK, nil
}

func (w *Worker) Execute() {
	defer func() {
		close(w.PacketChannel)
		w.Done = true // jobs is done
	}()

	slog.Info("executing worker", "address", w.Addr.String())

	timer := time.NewTimer(time.Duration(TIMEOUT) * time.Second)

	for {
		select {
		case <-timer.C: // after reach TIMEOUT
			return // simply stop working

		case buffer := <-w.PacketChannel:
			//fmt.Println("packet: ", buffer)
			switch int(buffer[0]) {
			case int(msg.Verb_REQUEST):
				fileBuffer, result, err := w.readFile(buffer[1:]) // skip identification byte
				// server errors
				switch result {
				case msg.Result_INVALID_PACKET_FORMAT:
					slog.Error("failed to decode buffer as protobuf message", "address", w.Addr.String(), "error", err)
					w.sendConfirmationPacket(result, "")
					return // exit worker

				case msg.Result_FILE_NOT_FOUND:
					slog.Error("failed to read file", "address", w.Addr.String(), "error", err)
					w.sendConfirmationPacket(result, "")
					return // exit worker

				case msg.Result_ERROR_CHECK_SUM:
					slog.Error("failed to generate sha256", "address", w.Addr.String(), "error", err)
					w.sendConfirmationPacket(result, "")
					return // exit worker

				case msg.Result_OK:
					slog.Info("start sending all file chunks packets", "address", w.Addr.String())
					w.tokenizeFile(fileBuffer)
					go w.sendFile() // start sending file chunks
					break
				default:
					return // wtf
				}

			case int(msg.Verb_CONFIRMATION):
				confirm := msg.Confirmation{}
				if err := pb.Unmarshal(buffer[1:], &confirm); err != nil {
					slog.Error("failed to decode buffer as protobuf message", "address", w.Addr.String(), "error", err)
					w.sendConfirmationPacket(msg.Result_INVALID_PACKET_FORMAT, "")
					return // exit worker
				}
				// reset timer to wait next confirmation messages
				timer.Reset(time.Duration(TIMEOUT) * time.Second)

				switch confirm.Result {
				case msg.Result_OK:
					w.Waiting = false // stop waiting confirmation packet
					if err := w.confirmPacket(confirm.Token); err != nil {
						slog.Error("cannot confirm packet, failed to find token hash", "address", w.Addr.String(), "token", confirm.Token, "error", err)
						w.sendConfirmationPacket(msg.Result_INVALID_TOKEN, confirm.Token)
					}
					slog.Info("received packet", "address", w.Addr.String(), "token", confirm.Token)
					break
				case msg.Result_PACKET_MISS:
					w.resendLastPacket(confirm.Token)
					break
				case msg.Result_VALID_CHECKSUM:
					slog.Info("checksum validation worked", "address", w.Addr.String())
					return // just stop working
				default:
					return // wtf
				}
			}

		default:
			//fmt.Println("waiting packet.....")
		}
	}
}
