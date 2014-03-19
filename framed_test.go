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

func TestReadToBytes(t *testing.T) {
	header := []byte("Header Header header Header")
	body := []byte("Body Body Body Body Body Body")
	buffer := CloseableBuffer{bytes.NewBuffer(make([]byte, 0))}
	fbuffer := NewFramed(buffer)
	if err := fbuffer.WriteFrame(header, body); err != nil {
		t.Fatalf("Unable to write: %s", err)
	}
	frame, err := fbuffer.ReadInitial()
	if err != nil {
		t.Fatalf("Unable to read frame: %s", err)
	}
	readHeader := make([]byte, len(header))
	readBody := make([]byte, len(body))
	if int(frame.headerLength) != len(header) {
		t.Errorf("Expected headerLength %d, got %d", len(header), frame.headerLength)
	}
	if int(frame.bodyLength) != len(body) {
		t.Errorf("Expected headerLength %d, got %d", len(body), frame.bodyLength)
	}
	if _, err = frame.ReadHeader(readHeader); err != nil {
		t.Fatalf("Unable to read header: %s", err)
	}
	if _, err = frame.ReadBody(readBody); err != nil {
		t.Fatalf("Unable to read body: %s", err)
	}
	if !bytes.Equal(readHeader, header) {
		t.Errorf("Header did not match expected.  Expected: '%s', Received: '%s'", string(header), string(readHeader))
	}
	if !bytes.Equal(readBody, body) {
		t.Errorf("Body did not match expected.  Expected: '%s', Received: '%s'", string(body), string(readBody))
	}
}

func TestReadToStreamWithCopy(t *testing.T) {
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
	frame.CopyTo(buffer2)
	frame, err = fbuffer2.ReadInitial()
	if err != nil {
		t.Fatalf("Unable to read frame: %s", err)
	}
	readHeader := bytes.NewBuffer(make([]byte, 0))
	readBody := bytes.NewBuffer(make([]byte, 0))
	if int(frame.headerLength) != len(header) {
		t.Errorf("Expected headerLength %d, got %d", len(header), frame.headerLength)
	}
	if int(frame.bodyLength) != len(body) {
		t.Errorf("Expected headerLength %d, got %d", len(body), frame.bodyLength)
	}
	if err = frame.CopyHeader(readHeader); err != nil {
		t.Fatalf("Unable to read header: %s", err)
	}
	if err = frame.CopyBody(readBody); err != nil {
		t.Fatalf("Unable to read body: %s", err)
	}
	if !bytes.Equal(readHeader.Bytes(), header) {
		t.Errorf("Header did not match expected.  Expected: '%s', Received: '%s'", string(header), string(readHeader.Bytes()))
	}
	if !bytes.Equal(readBody.Bytes(), body) {
		t.Errorf("Body did not match expected.  Expected: '%s', Received: '%s'", string(body), string(readBody.Bytes()))
	}
}
