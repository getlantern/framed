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
A Framed enhances an io.ReadWriteCloser to write data in contiguous frames.

It implements the Reader and Writer interfaces, but unlike typical Readers
it only returns whole frames.  Unlike typical Writers, it will not allow
frames to be fragmented.

Although the underlying ReadWriteCloser may be safe to use from multiple
goroutines, a Framed is not.
*/
type Framed struct {
	Stream io.ReadWriteCloser // the raw underlying connection

}

func (framed *Framed) Close() error {
	return framed.Stream.Close()
}

/*
Read implements the function from io.Reader.  Unlike io.Reader.Read,
frame.Read only returns full frames of data.
*/
func (framed *Framed) Read(buffer []byte) (n int, err error) {
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
header that allows the Framed on the other end to read the whole frame.
*/
func (framed *Framed) Write(frame []byte) (n int, err error) {
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
