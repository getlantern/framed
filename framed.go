/*
Package framed provides an implementation of io.ReadWriteCloser that reads and
writes whole frames only.

Frames are length-prefixed.  The first two bytes are an unsigned 16 bit int
stored in little-endian byte order.  The remaining bytes are the actual content
of the frame.

The use of a uint16 means that the maximum possible frame length is 65535.
*/
package framed

import (
	"encoding/binary"
	"fmt"
	"io"
)

var endianness = binary.LittleEndian

/*
A FramedReader enhances an io.ReadCloser to read data in contiguous frames.
It implements the io.Reader interface, but nlike typical Readers it only returns
whole frames.  Unlike typical Writers, it will not allow frames to be
fragmented.

Although the underlying stream may be safe to use from multiple
goroutines, a FramedReader is not.
*/
type FramedReader struct {
	Stream io.Reader // the raw underlying connection
}

/*
A FramedWriter enhances an io.WriteCLoser to write data in contiguous frames.
It implements the io.Writer interface, but unlike typical Writers, it includes
information that allows a corresponding FramedReader to read whole frames
without them being fragmented.

Although the underlying stream may be safe to use from multiple
goroutines, a FramedWriter is not.
*/
type FramedWriter struct {
	Stream io.Writer // the raw underlying connection
}

/*
Read implements the function from io.Reader.  Unlike io.Reader.Read,
frame.Read only returns full frames of data (assuming that the data was written
by a FramedWriter).
*/
func (framed *FramedReader) Read(buffer []byte) (n int, err error) {
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
	var totalRead int
	for totalRead = 0; totalRead < n; {
		var nr int
		nr, err = framed.Stream.Read(buffer[totalRead:n])
		if err != nil {
			return
		}
		totalRead += nr
	}
	if totalRead != n {
		err = fmt.Errorf("%d bytes read did not match %d bytes expected", totalRead, n)
	}
	return
}

/*
Write implements the Write method from io.Writer.  It prepends a frame length
header that allows the FramedReader on the other end to read the whole frame.
*/
func (framed *FramedWriter) Write(frame []byte) (n int, err error) {
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
