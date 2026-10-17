package lojbo

import (
	"bytes"
	"fmt"
	"os"

	"genturfa_i/bynyfys"
)

// Load reads an EBNF grammar in the notation used by "bynyfys pa". In that
// notation <...> marks an elidable terminator; here it becomes an ordinary
// optional part, exactly as the CLL grammar intends.
func Load(path string) (*Parser, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("fliba lonu tcidu - %w", err)
	}
	data = bytes.ReplaceAll(data, []byte{'<'}, []byte{'['})
	data = bytes.ReplaceAll(data, []byte{'>'}, []byte{']'})
	g, err := ebnf.Parse(path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("fliba lonu genturfa'i - %w", err)
	}
	if err := ebnf.Verify(g, "text"); err != nil {
		return nil, fmt.Errorf("fliba lonu cipcta - %w", err)
	}
	return New(g)
}
