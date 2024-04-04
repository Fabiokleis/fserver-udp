package File

import (
	"fmt"
	"os"
)

func TokenHash(idx int) string {
	return fmt.Sprintf("token_idx_%d", idx)
}

type Token struct {
	Index    int
	Received bool
}

type TokenizableFile struct {
	FileName string
	File     *os.File // file
	Size     int      // file content amount of bytes
	Tokens   []*Token // tokenized indexes
	CheckSum string   // file checksum
}

func (tF *TokenizableFile) Close() {
	if tF.File != nil {
		tF.File.Close()
	}
}

func (tF *TokenizableFile) DeleteFile() error {
	tF.Close()
	return os.Remove(tF.FileName)
}

func (tF *TokenizableFile) CreateFile(name string) error {

	f, err := os.OpenFile(name, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return err
	}

	tF.FileName = name
	tF.File = f

	return nil
}

func (tF *TokenizableFile) Path() string {
	return tF.File.Name()
}

func (tF *TokenizableFile) ReadChunk(offset int, chunkSize int) []byte {
	chunk := make([]byte, chunkSize)

	n, _ := tF.File.ReadAt(chunk, int64(offset))

	return chunk[:n]
}

func (tF *TokenizableFile) WriteChunk(chunk []byte) {
	tF.Size += len(chunk)
	tF.File.Write(chunk)
}

func (tF *TokenizableFile) FindToken(hash string) *Token {
	for _, t := range tF.Tokens {
		if TokenHash(t.Index) == hash {
			return t
		}
	}
	return nil
}

func (tF *TokenizableFile) FindNotReceivedToken() *Token {
	for _, t := range tF.Tokens {
		if !t.Received {
			return t
		}
	}
	return nil
}

func (tF *TokenizableFile) PushToken(idx int, received bool) {
	tF.Tokens = append(tF.Tokens, &Token{Index: idx, Received: received})
}
