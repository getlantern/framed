package framed

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

const (
	sleepTime = 5 * time.Millisecond
)

type CloseableBuffer struct {
	raw *bytes.Buffer
}

func (buffer CloseableBuffer) Read(data []byte) (n int, err error) {
	// Sleep a bit after reading to surface concurrency issues
	defer time.Sleep(sleepTime)
	return buffer.raw.Read(data)
}

func (buffer CloseableBuffer) Write(data []byte) (n int, err error) {
	// Sleep a bit after writing to surface concurrency issues
	defer time.Sleep(sleepTime)
	return buffer.raw.Write(data)
}

func (buffer CloseableBuffer) Close() (err error) {
	return
}

func TestFraming(t *testing.T) {
	testMessage := []byte("This is a test message")
	piece1 := testMessage[:8]
	piece2 := testMessage[8:]
	cb := CloseableBuffer{bytes.NewBuffer(make([]byte, 0))}
	defer cb.Close()
	writer := NewWriter(cb)
	reader := NewReader(cb)

	// Do a bunch of concurrent reads and writes to make sure we're threadsafe
	iters := 100
	var wg sync.WaitGroup
	for i := 0; i < iters; i++ {
		wg.Add(2)
		writePieces := i%2 == 0
		readFrame := i%3 == 0

		go func() {
			// Write
			var n int
			var err error
			if writePieces {
				n, err = writer.WritePieces(piece1, piece2)
			} else {
				n, err = writer.Write(testMessage)
			}
			if err != nil {
				t.Errorf("Unable to write: %s", err)
			} else if n != len(testMessage) {
				t.Errorf("%d bytes written did not match length of test message %d", n, len(testMessage))
			}
			wg.Done()
		}()

		go func() {
			// Read
			var frame []byte
			var n int
			var err error
			buffer := make([]byte, 100)

			if readFrame {
				if frame, err = reader.ReadFrame(); err != nil {
					t.Errorf("Unable to read frame: %s", err)
					return
				}
			} else {
				if n, err = reader.Read(buffer); err != nil {
					t.Errorf("Unable to read: %s", err)
					return
				} else if n != len(testMessage) {
					t.Errorf("%d bytes read did not match length of test message %d", n, len(testMessage))
					return
				}
				frame = buffer[:n]
			}

			if !bytes.Equal(frame, testMessage) {
				t.Errorf("Received did not match expected.  Expected: '%q', Received: '%q'", testMessage, frame)
			}
			wg.Done()
		}()
	}

	wg.Wait()
}
