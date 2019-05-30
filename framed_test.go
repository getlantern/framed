package framed

import (
	"io"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/getlantern/testify/assert"
)

const (
	sleepTime = 5 * time.Millisecond
)

type pipe struct {
	io.Reader
	io.WriteCloser
}

func newPipe() io.ReadWriteCloser {
	r, w := io.Pipe()
	return &pipe{r, w}
}

func (p *pipe) Read(data []byte) (n int, err error) {
	// Sleep a bit after reading to surface concurrency issues
	defer time.Sleep(sleepTime)
	return p.Reader.Read(data)
}

func (p *pipe) Write(data []byte) (n int, err error) {
	// Sleep a bit after writing to surface concurrency issues
	defer time.Sleep(sleepTime)
	return p.WriteCloser.Write(data)
}

func TestSmallFrames(t *testing.T) {
	doTestFrames(t, 20)
}

func TestBigFrames(t *testing.T) {
	doTestFrames(t, MaxFrameLength+1)
}

func doTestFrames(t *testing.T, msgLength int) {
	pool := NewHeaderPreservingBufferPool(1, msgLength, msgLength > MaxFrameLength)
	testMessage := pool.GetSlice()
	defer pool.PutSlice(testMessage)
	rand.Read(testMessage.Bytes())

	cutoff := len(testMessage.Bytes()) / 2
	piece1 := testMessage.Bytes()[:cutoff]
	piece2 := testMessage.Bytes()[cutoff:]

	p := newPipe()
	defer p.Close()
	writer := NewWriter(p)
	reader := NewReader(p)
	if msgLength > MaxFrameLength {
		writer.EnableBigFrames()
		reader.EnableBigFrames()
	}
	reader.EnableBuffering(msgLength)

	// Do a bunch of concurrent reads and writes to make sure we're threadsafe
	iters := 100
	writersDone := make(chan bool, iters)
	readersDone := make(chan bool, iters)

	for i := 0; i < iters; i++ {
		writePieces := i%2 == 0
		readFrame := i%3 == 0
		writeAtomic := !writePieces && i%5 == 0

		go func() {
			defer func() {
				writersDone <- true
			}()

			// Write
			var n int
			var err error
			if writePieces {
				n, err = writer.WritePieces(piece1, piece2)
			} else if writeAtomic {
				n, err = writer.WriteAtomic(testMessage)
			} else {
				n, err = writer.Write(testMessage.Bytes())
			}
			if err != nil {
				t.Errorf("Unable to write: %s", err)
			} else {
				assert.Equal(t, len(testMessage.Bytes()), n, "Bytes written should match length of test message")
			}
		}()

		go func() {
			defer func() {
				readersDone <- true
			}()

			// Read
			var frame []byte
			var n int
			var err error
			buffer := make([]byte, len(testMessage.Bytes()))

			if readFrame {
				if frame, err = reader.ReadFrame(); err != nil {
					t.Errorf("Unable to read frame: %s", err)
					return
				}
			} else {
				if n, err = reader.Read(buffer); err != nil {
					t.Errorf("Unable to read: %s", err)
					return
				} else {
					assert.Equal(t, len(testMessage.Bytes()), n, "Bytes read should match length of test message")
				}
				frame = buffer[:n]
			}

			assert.Equal(t, testMessage.Bytes(), frame, "Received should match sent")
		}()
	}

	timeout := time.After(10 * time.Second)
	for i := 0; i < iters; i++ {
		select {
		case <-writersDone:
			// good
		case <-timeout:
			t.Fatalf("Gave up waiting for writers after %d", i)
		}
	}

	timeout = time.After(10 * time.Second)
	for i := 0; i < iters; i++ {
		select {
		case <-readersDone:
			// good
		case <-timeout:
			t.Fatalf("Gave up waiting for readers after %d", i)
		}
	}
}

func TestWriteTooLong(t *testing.T) {
	w := NewWriter(ioutil.Discard)
	b := make([]byte, MaxFrameLength+1)
	n, err := w.Write(b)
	assert.Error(t, err, "Writing too long message should result in error")
	assert.Equal(t, 0, n, "Writing too long message should result in 0 bytes written")
	n, err = w.Write(b[:len(b)-1])
	assert.NoError(t, err, "Writing message of MaxFrameLength should be allowed")
	assert.Equal(t, MaxFrameLength, n, "Writing message of MaxFrameLength should have written MaxFrameLength bytes")
}

func TestWritePiecesTooLong(t *testing.T) {
	w := NewWriter(ioutil.Discard)
	b1 := make([]byte, MaxFrameLength)
	b2 := make([]byte, 1)
	n, err := w.WritePieces(b1, b2)
	assert.Error(t, err, "Writing too long message should result in error")
	assert.Equal(t, 0, n, "Writing too long message should result in 0 bytes written")
	n, err = w.WritePieces(b1[:len(b1)-1], b2)
	assert.NoError(t, err, "Writing message of MaxFrameLength should be allowed")
	assert.Equal(t, MaxFrameLength, n, "Writing message of MaxFrameLength should have written MaxFrameLength bytes")
}
