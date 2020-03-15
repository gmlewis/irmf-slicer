package stl

import (
	"fmt"
	"testing"
)

func TestWriter(t *testing.T) {
	tests := []struct {
		name string
		tris []Tri
	}{
		{
			name: "no triangles",
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test #%v: %v", i, tt.name), func(t *testing.T) {
			out := &fakeFile{}
			ch := make(chan Tri, bufSize)
			c := &Client{ch: ch}
			c.start(out)

			for i, tri := range tt.tris {
				if err := c.Write(&tri); err != nil {
					t.Fatalf("c.Write: i=%v, %v", i, err)
				}
			}
			if err := c.Close(); err != nil {
				t.Fatalf("c.Close: %v", err)
			}

			if out.closes != 1 {
				t.Errorf("expected 1 close, got %v", out.closes)
			}
			if out.seeks != 1 {
				t.Errorf("expected 1 seek, got %v", out.seeks)
			}
			if out.writes != len(tt.tris)+1 { // +1 for the final count
				t.Errorf("expected %v writes, got %v", len(tt.tris), out.writes)
			}
		})
	}
}

type fakeFile struct {
	closes int
	seeks  int
	writes int
}

func (f *fakeFile) Close() error {
	f.closes++
	return nil
}

func (f *fakeFile) Seek(offset int64, whence int) (int64, error) {
	f.seeks++
	return 0, nil
}

func (f *fakeFile) Write(p []byte) (n int, err error) {
	f.writes++
	return 0, nil
}
