package env

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/src-d/go-billy.v4/memfs"
)

func TestTempDir(t *testing.T) {
	mfs := memfs.New()
	mfs.MkdirAll("/tmp", os.ModePerm)
	td, err := tempDir(mfs, "", "foo-")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if !strings.HasPrefix(td, "/tmp/foo-") {
		t.Fatalf("expected prefix: %v", td)
	}
}
