package api

import (
	"log"
	"os"
	"testing"

	"github.com/dollarshaveclub/acyl/pkg/testhelper/localdb"
)

func TestMain(m *testing.M) {
	var exit int
	defer func() { os.Exit(exit) }()
	ldb := localdb.New(log.Printf)
	ldb.MustRun()
	defer ldb.Stop()
	exit = m.Run()
}
