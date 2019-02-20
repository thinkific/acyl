package metahelm

import (
	"testing"

	"gopkg.in/src-d/go-billy.v4/memfs"
)

func TestReadFileSafely(t *testing.T) {
	oldmax := fileSizeMaxBytes
	defer func() { fileSizeMaxBytes = oldmax }()
	fileSizeMaxBytes = 5
	mfs := memfs.New()
	f, _ := mfs.Create("foo.txt")
	f.Write([]byte("1234567890"))
	if _, err := readFileSafely(mfs, "foo.txt"); err == nil {
		t.Fatalf("should have failed")
	}
	fileSizeMaxBytes = 100
	d, err := readFileSafely(mfs, "foo.txt")
	if err != nil {
		t.Fatalf("should have succeeded: %v", err)
	}
	if string(d) != "1234567890" {
		t.Fatalf("bad contents: %v", string(d))
	}
}
