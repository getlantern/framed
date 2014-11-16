/*
Package framed provides an implementation of io.ReadWriteCloser that reads and
writes whole frames only.

Frames are length-prefixed.  The first two bytes are an unsigned 16 bit int
stored in little-endian byte order indicating the length of the content.  The
remaining bytes are the actual content of the frame.

The use of a uint16 means that the maximum possible frame size (MAX_FRAME_SIZE)
is 65535.
*/
package framed

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

const (
	// MAX_FRAME_SIZE is the maximum possible size of a frame (not including the
	// length prefix)
	MAX_FRAME_SIZE = 65535
)

var endianness = binary.LittleEndian

/*
A framed.Reader enhances an io.ReadCloser to read data in contiguous frames.
It implements the io.Reader interface, but nlike typical Readers it only returns
whole frames.  Unlike typical Writers, it will not allow frames to be
fragmented.

Although the underlying stream may be safe to use from multiple
goroutines, a framed.Reader is not.
*/
type Reader struct {
	Stream io.Reader // the raw underlying connection
	mutex  sync.Mutex
}

/*
A framed.Writer enhances an io.WriteCLoser to write data in contiguous frames.
It implements the io.Writer interface, but unlike typical Writers, it includes
information that allows a corresponding framed.Reader to read whole frames
without them being fragmented.

A framed.Writer also supports a method that writes multiple buffers to the
underlying stream as a single frame.
*/
type Writer struct {
	Stream io.Writer // the raw underlying connection
	mutex  sync.Mutex
}

func NewReader(r io.Reader) *Reader {
	return &Reader{Stream: r}
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{Stream: w}
}

/*
Read implements the function from io.Reader.  Unlike io.Reader.Read,
frame.Read only returns full frames of data (assuming that the data was written
by a framed.Writer).
*/
func (framed *Reader) Read(buffer []byte) (n int, err error) {
	framed.mutex.Lock()
	defer framed.mutex.Unlock()

	var nb uint16
	err = binary.Read(framed.Stream, endianness, &nb)
	if err != nil {
		return
	}

	n = int(nb)

	bufferSize := len(buffer)
	if n > bufferSize {
		return 0, fmt.Errorf("Buffer of size %d is too small to hold frame of size %d", bufferSize, n)
	}

	// Read into buffer
	n, err = io.ReadFull(framed.Stream, buffer[:n])
	return
}

/*
Write implements the Write method from io.Writer.  It prepends a frame length
header that allows the framed.Reader on the other end to read the whole frame.
*/
func (framed *Writer) Write(frame []byte) (n int, err error) {
	framed.mutex.Lock()
	defer framed.mutex.Unlock()

	n = len(frame)

	// Write the length header
	if err = binary.Write(framed.Stream, endianness, uint16(n)); err != nil {
		return
	}

	// Write the data
	var written int
	if written, err = framed.Stream.Write(frame); err != nil {
		return
	}
	if written != n {
		err = fmt.Errorf("%d bytes written, expected to write %d", written, n)
	}
	return
}

func (framed *Writer) WritePieces(pieces ...[]byte) (n int, err error) {
	framed.mutex.Lock()
	defer framed.mutex.Unlock()

	for _, piece := range pieces {
		n = n + len(piece)
	}

	// Write the length header
	if err = binary.Write(framed.Stream, endianness, uint16(n)); err != nil {
		return
	}

	// Write the data
	var written int
	for _, piece := range pieces {
		var nw int
		if nw, err = framed.Stream.Write(piece); err != nil {
			return
		}
		written = written + nw
	}
	if written != n {
		err = fmt.Errorf("%d bytes written, expected to write %d", written, n)
	}
	return
}
