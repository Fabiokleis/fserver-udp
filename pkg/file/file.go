package File

import "fmt"

type Token struct {
	Index    int
	Received bool
}

type TokenizableFile struct {
	Buffer   []byte   // file content
	Size     int      // file content amount of bytes
	Tokens   []*Token // tokenized indexes
	CheckSum string   // file checksum
}

func TokenHash(idx int) string {
	return fmt.Sprintf("token_idx_%d", idx)
}

func (tF *TokenizableFile) FindToken(hash string) *Token {
	for _, t := range tF.Tokens {
		if TokenHash(t.Index) == hash {
			return t
		}
	}
	return nil
}

func (tF *TokenizableFile) PushToken(idx int, received bool) {
	tF.Tokens = append(tF.Tokens, &Token{Index: idx, Received: received})
}
