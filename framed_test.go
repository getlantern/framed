package framed

import (
	"bytes"
	"github.com/oxtoacart/bpool"
	"testing"
)

type CloseableBuffer struct {
	raw *bytes.Buffer
}

func (buffer CloseableBuffer) Read(data []byte) (n int, err error) {
	return buffer.raw.Read(data)
}

func (buffer CloseableBuffer) Write(data []byte) (n int, err error) {
	return buffer.raw.Write(data)
}

func (buffer CloseableBuffer) Close() (err error) {
	return
}

func TestFraming(t *testing.T) {
	msgPart1 := []byte("This is a ")
	msgPart2 := []byte("test message")
	testMessage := []byte("This is a test message")
	fbuffer := NewFramed(CloseableBuffer{bytes.NewBuffer(make([]byte, 0))}, bpool.NewBytePool(100, 5))
	if err := fbuffer.WriteFrame(msgPart1, msgPart2); err != nil {
		t.Errorf("Unable to write: %s", err)
	}
	if frame, err := fbuffer.ReadFrame(); err != nil {
		t.Errorf("Unable to read: %s", err)
	} else {
		//defer frame.Release()
		received := bytes.Join(frame.Buffers, nil)
		if !bytes.Equal(received, testMessage) {
			t.Errorf("Received did not match expected.  Expected: '%s', Received: '%s'", string(testMessage), string(received))
		}
	}
}
