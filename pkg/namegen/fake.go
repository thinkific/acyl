package namegen

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/pkg/errors"
)

type FakeNameGenerator struct {
	Prefix string
	Unique bool
}

func (fng FakeNameGenerator) New() (string, error) {
	var sfx string
	if fng.Unique {
		rn, err := rand.Int(rand.Reader, big.NewInt(99999))
		if err != nil {
			return "", errors.Wrap(err, "error getting random integer")
		}
		sfx = fmt.Sprintf("-%d", rn)
	}
	return fng.Prefix + "fake-name" + sfx, nil
}
