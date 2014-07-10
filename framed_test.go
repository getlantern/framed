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
	cb := CloseableBuffer{bytes.NewBuffer(make([]byte, 0))}
	writer := NewWriter(cb)
	reader := &Reader{cb}
	defer cb.Close()

	// Write
	if n, err := writer.Write(testMessage); err != nil {
		t.Errorf("Unable to write: %s", err)
	} else if n != len(testMessage) {
		t.Errorf("%d bytes written did not match length of test message %d", n, len(testMessage))
	}

	// Read
	buffer := make([]byte, 100)
	if n, err := reader.Read(buffer); err != nil {
		t.Errorf("Unable to read: %s", err)
	} else if n != len(testMessage) {
		t.Errorf("%d bytes read did not match length of test message %d", n, len(testMessage))
	} else {
		if !bytes.Equal(buffer[:n], testMessage) {
			t.Errorf("Received did not match expected.  Expected: '%s', Received: '%s'", string(testMessage), string(buffer[:n]))
		}
	}
}
