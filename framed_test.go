package framed

import (
	"bytes"
	"io/ioutil"
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

func TestWriteAndRead(t *testing.T) {
	header := []byte("Header Header header Header")
	body := []byte("Body Body Body Body Body Body")
	buffer := CloseableBuffer{bytes.NewBuffer(make([]byte, 0))}
	fbuffer := NewFramed(buffer)
	buffer2 := CloseableBuffer{bytes.NewBuffer(make([]byte, 0))}
	fbuffer2 := NewFramed(buffer2)
	if err := fbuffer.WriteFrame(header, body); err != nil {
		t.Fatalf("Unable to write: %s", err)
	}
	frame, err := fbuffer.ReadInitial()
	if err != nil {
		t.Fatalf("Unable to read frame: %s", err)
	}
	if err = frame.CopyTo(buffer2); err != nil {
		t.Fatalf("Unable to copy frame")
	}
	if frame, err = fbuffer2.ReadInitial(); err != nil {
		t.Fatalf("Unable to read initial frame from copy: %s", err)
	}
	if int(frame.HeaderLength()) != len(header) {
		t.Errorf("Expected headerLength %d, got %d", len(header), frame.HeaderLength())
	}
	if int(frame.BodyLength()) != len(body) {
		t.Errorf("Expected headerLength %d, got %d", len(body), frame.BodyLength())
	}
	readHeader, err := ioutil.ReadAll(frame.Header())
	if err != nil {
		t.Fatalf("Error reading header: %s", err)
	}
	readBody, err := ioutil.ReadAll(frame.Body())
	if err != nil {
		t.Fatalf("Error reading body: %s", err)
	}
	if !bytes.Equal(readHeader, header) {
		t.Errorf("Header did not match expected.  Expected: '%s', Received: '%s'", string(header), string(readHeader))
	}
	if !bytes.Equal(readBody, body) {
		t.Errorf("Body did not match expected.  Expected: '%s', Received: '%s'", string(body), string(readBody))
	}
}
