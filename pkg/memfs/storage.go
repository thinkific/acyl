package memfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type storage struct {
	sync.RWMutex
	files    map[string]*file
	children map[string]map[string]*file
}

func newStorage() *storage {
	return &storage{
		files:    make(map[string]*file, 0),
		children: make(map[string]map[string]*file, 0),
	}
}

// Has read locks
func (s *storage) Has(path string) bool {
	s.RLock()
	defer s.RUnlock()
	return s.has(path)
}

// has assumes a read lock
func (s *storage) has(path string) bool {
	path = clean(path)
	_, ok := s.files[path]
	return ok
}

// New locks
func (s *storage) New(path string, mode os.FileMode, flag int) (*file, error) {
	s.Lock()
	defer s.Unlock()
	return s.new(path, mode, flag)
}

// new assumes a lock is held
func (s *storage) new(path string, mode os.FileMode, flag int) (*file, error) {
	path = clean(path)
	if s.has(path) {
		if !s.mustGet(path).mode.IsDir() {
			return nil, fmt.Errorf("file already exists %q", path)
		}

		return nil, nil
	}

	f := &file{
		name:    filepath.Base(path),
		content: &content{},
		mode:    mode,
		flag:    flag,
	}

	s.files[path] = f
	s.createParent(path, mode, f)
	return f, nil
}

// createParent assumes that a lock is already being held by the goroutine calling it
func (s *storage) createParent(path string, mode os.FileMode, f *file) error {
	base := filepath.Dir(path)
	base = clean(base)
	if f.Name() == string(separator) {
		return nil
	}

	if _, err := s.new(base, mode.Perm()|os.ModeDir, 0); err != nil {
		return err
	}

	if _, ok := s.children[base]; !ok {
		s.children[base] = make(map[string]*file, 0)
	}

	s.children[base][f.Name()] = f
	return nil
}

func (s *storage) Children(path string) []*file {
	s.RLock()
	defer s.RUnlock()
	path = clean(path)

	l := make([]*file, 0)
	for _, f := range s.children[path] {
		l = append(l, f)
	}

	return l
}

// MustGet read locks
func (s *storage) MustGet(path string) *file {
	s.RLock()
	defer s.RUnlock()
	return s.mustGet(path)
}

// mustGet assumes a read lock is held
func (s *storage) mustGet(path string) *file {
	f, ok := s.get(path)
	if !ok {
		panic(fmt.Errorf("couldn't find %q", path))
	}

	return f
}

// Get read locks
func (s *storage) Get(path string) (*file, bool) {
	s.RLock()
	defer s.RUnlock()
	return s.get(path)
}

// get assumes a read lock is held
func (s *storage) get(path string) (*file, bool) {
	path = clean(path)
	if !s.has(path) {
		return nil, false
	}

	file, ok := s.files[path]
	return file, ok
}

func (s *storage) Rename(from, to string) error {
	from = clean(from)
	to = clean(to)

	if !s.Has(from) {
		return os.ErrNotExist
	}

	move := [][2]string{{from, to}}

	for pathFrom := range s.files {
		if pathFrom == from || !filepath.HasPrefix(pathFrom, from) {
			continue
		}

		rel, _ := filepath.Rel(from, pathFrom)
		pathTo := filepath.Join(to, rel)

		move = append(move, [2]string{pathFrom, pathTo})
	}

	for _, ops := range move {
		from := ops[0]
		to := ops[1]

		if err := s.move(from, to); err != nil {
			return err
		}
	}

	return nil
}

func (s *storage) move(from, to string) error {
	s.Lock()
	defer s.Unlock()
	s.files[to] = s.files[from]
	s.files[to].name = filepath.Base(to)
	s.children[to] = s.children[from]

	defer func() {
		delete(s.children, from)
		delete(s.files, from)
		delete(s.children[filepath.Dir(from)], filepath.Base(from))
	}()

	return s.createParent(to, 0644, s.files[to])
}

func (s *storage) Remove(path string) error {
	path = clean(path)

	f, has := s.Get(path)
	if !has {
		return os.ErrNotExist
	}

	s.RLock()
	if f.mode.IsDir() && len(s.children[path]) != 0 {
		s.RUnlock()
		return fmt.Errorf("dir: %s contains files", path)
	}
	s.RUnlock()

	base, file := filepath.Split(path)
	base = filepath.Clean(base)

	s.Lock()
	delete(s.children[base], file)
	delete(s.files, path)
	s.Unlock()
	return nil
}

func clean(path string) string {
	return filepath.Clean(filepath.FromSlash(path))
}

type content struct {
	sync.RWMutex
	bytes []byte
}

func (c *content) WriteAt(p []byte, off int64) (int, error) {
	c.Lock()
	defer c.Unlock()
	prev := len(c.bytes)

	diff := int(off) - prev
	if diff > 0 {
		c.bytes = append(c.bytes, make([]byte, diff)...)
	}

	c.bytes = append(c.bytes[:off], p...)
	if len(c.bytes) < prev {
		c.bytes = c.bytes[:prev]
	}

	return len(p), nil
}

func (c *content) ReadAt(b []byte, off int64) (n int, err error) {
	c.RLock()
	defer c.RUnlock()
	size := int64(len(c.bytes))
	if off >= size {
		return 0, io.EOF
	}

	l := int64(len(b))
	if off+l > size {
		l = size - off
	}

	btr := c.bytes[off : off+l]
	if len(btr) < len(b) {
		err = io.EOF
	}
	n = copy(b, btr)

	return
}
