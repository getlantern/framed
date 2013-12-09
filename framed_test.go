package framed

import (
	"bytes"
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
	testMessage := []byte("This is a test message")
	buffer := CloseableBuffer{bytes.NewBuffer(make([]byte, 0))}
	fbuffer := Framed{buffer}
	if err := fbuffer.WriteFrame(testMessage); err != nil {
		t.Errorf("Unable to write: %s", err)
	}
	if receivedMsg, err := fbuffer.ReadFrame(); err != nil {
		t.Errorf("Unable to read: %s", err)
	} else {
		if !bytes.Equal(receivedMsg, testMessage) {
			t.Errorf("Received did not match expected.  Expected: '%s', Received: '%s'", string(testMessage), string(receivedMsg))
		}
	}
}
