// Package stl provides a streaming binary STL file writer.
package stl

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
)

const (
	headerSize = 80
	bufSize    = 10000
)

// Client is a streaming binary STL file writer client.
type Client struct {
	wg sync.WaitGroup // ensures file is closed
	ch chan Tri

	mu  sync.RWMutex
	err error
}

// Tri represents an STL triangle.
type Tri struct {
	// Normal plus three vertex triplets: [3]float{x,y,z}
	N, V1, V2, V3 [3]float32
	_             uint16 // unused attribute byte count
}

// New creates a new streaming binary STL file writer.
func New(filename string) (*Client, error) {
	out, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	// Write header
	header := struct {
		_ [headerSize]uint8
		_ uint32 // count will be overwritten on channel close.
	}{}
	if err := binary.Write(out, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("error writing header: %v", err)
	}

	ch := make(chan Tri, bufSize)
	c := &Client{
		ch: ch,
	}
	c.start(out)
	return c, nil
}

func (c *Client) start(out writeSeekCloser) {
	c.wg.Add(1)
	go func() {
		err := writer(out, c.ch)
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		c.wg.Done()
	}()
}

// Write writes a triangle to the STL file.
func (c *Client) Write(t *Tri) error {
	c.ch <- *t
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.err
}

// Close finalizes the STL file.
func (c *Client) Close() error {
	close(c.ch)
	c.wg.Wait()
	return c.err
}

type writeSeekCloser interface {
	io.Writer
	io.Seeker
	io.Closer
}

func writer(out writeSeekCloser, ch <-chan Tri) error {
	var count uint32
	for t := range ch {
		if err := binary.Write(out, binary.LittleEndian, &t); err != nil {
			return fmt.Errorf("write triangle %#v: %v", t, err)
		}
		count++
	}

	if _, err := out.Seek(headerSize, io.SeekStart); err != nil {
		return fmt.Errorf("seek: %v", err)
	}

	if err := binary.Write(out, binary.LittleEndian, &count); err != nil {
		return fmt.Errorf("write count %v: %v", count, err)
	}

	return out.Close()
}
