package remotetest

import (
	"bytes"
	"context"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/petrusjohannesmaas/esque/internal/remote"
)

// Memory is an in-memory remote.Store for tests.
type Memory struct {
	Prefix   string
	Now      func() time.Time
	FailPut  error
	FailList error

	objects map[string]object
}

type object struct {
	data     []byte
	modified time.Time
}

// NewMemory returns an empty store under prefix.
func NewMemory(prefix string) *Memory {
	return &Memory{Prefix: prefix, objects: map[string]object{}}
}

// WithPrefix returns a store on the same objects under another prefix.
func (m *Memory) WithPrefix(prefix string) *Memory {
	return &Memory{Prefix: prefix, Now: m.Now, FailPut: m.FailPut, FailList: m.FailList, objects: m.objects}
}

// SetModified sets the modified time List reports for an object.
func (m *Memory) SetModified(name string, modified time.Time) {
	key := m.key(name)
	obj, ok := m.objects[key]
	if !ok {
		return
	}
	obj.modified = modified
	m.objects[key] = obj
}

func (m *Memory) key(name string) string {
	if m.Prefix == "" {
		return name
	}
	return strings.TrimSuffix(m.Prefix, "/") + "/" + name
}

func (m *Memory) folder() string {
	if m.Prefix == "" {
		return ""
	}
	return strings.TrimSuffix(m.Prefix, "/") + "/"
}

func (m *Memory) Put(_ context.Context, name string, r io.Reader, _ int64) error {
	if m.FailPut != nil {
		return m.FailPut
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	now := time.Now
	if m.Now != nil {
		now = m.Now
	}
	m.objects[m.key(name)] = object{data: data, modified: now()}
	return nil
}

func (m *Memory) Exists(_ context.Context, name string) (bool, error) {
	_, ok := m.objects[m.key(name)]
	return ok, nil
}

func (m *Memory) List(context.Context) ([]remote.Object, error) {
	if m.FailList != nil {
		return nil, m.FailList
	}
	folder := m.folder()
	list := make([]remote.Object, 0, len(m.objects))
	for key, obj := range m.objects {
		if !strings.HasPrefix(key, folder) {
			continue
		}
		name := strings.TrimPrefix(key, folder)
		if name == "" || strings.Contains(name, "/") {
			continue
		}
		list = append(list, remote.Object{Name: name, Size: int64(len(obj.data)), Modified: obj.modified})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

func (m *Memory) Get(_ context.Context, name string) (io.ReadCloser, error) {
	obj, ok := m.objects[m.key(name)]
	if !ok {
		return nil, remote.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(obj.data)), nil
}
