package namegen

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
)

const (
	legalDNSChars = "abcdefghijklmnopqrstuvwxyz-ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// NameGenerator describes an object capable of generating new environment names
type NameGenerator interface {
	New() (string, error)
}

// Wordset contains synsets for POS taken from WordNet
type Wordset struct {
	Adjective []string `json:"adjective"`
	Noun      []string `json:"noun"`
}

// WordnetNameGenerator generates names from WordNet data
type WordnetNameGenerator struct {
	ws     *Wordset
	logger *log.Logger
}

// NewWordnetNameGenerator loads filename (must be a gzip & JSON-encoded wordset list)
// and returns a WordnetNameGenerator
func NewWordnetNameGenerator(filename string, logger *log.Logger) (*WordnetNameGenerator, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	wng := &WordnetNameGenerator{
		ws:     &Wordset{},
		logger: logger,
	}
	d := json.NewDecoder(gz)
	err = d.Decode(&wng.ws)
	if err != nil {
		return nil, err
	}
	return wng, nil
}

// filter replaces whitespace and underscores and removes any non-DNS-compliant characters
func (wng WordnetNameGenerator) filter(name string) string {
	mf := func(r rune) rune {
		switch {
		case r == ' ' || r == '_':
			return '-'
		case strings.Contains(legalDNSChars, string(r)):
			return r
		default:
			return rune(-1)
		}
	}
	return strings.Map(mf, name)
}

// New returns a randomly-generated name of the form {adjective}-{noun}
func (wng *WordnetNameGenerator) New() (string, error) {
	ai, err := RandomRange(int64(len(wng.ws.Adjective)))
	if err != nil {
		return "", fmt.Errorf("error getting random index for adjective: %v", err)
	}
	ni, err := RandomRange(int64(len(wng.ws.Noun)))
	if err != nil {
		return "", fmt.Errorf("error getting random index for noun: %v", err)
	}
	return wng.filter(fmt.Sprintf("%v-%v", wng.ws.Adjective[ai], wng.ws.Noun[ni])), nil
}

// RandomRange returns a random integer (using rand.Reader as the entropy source) between 0 and max
func RandomRange(max int64) (int64, error) {
	maxBig := *big.NewInt(max)
	n, err := rand.Int(rand.Reader, &maxBig)
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}
