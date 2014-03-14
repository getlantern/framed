/*
Package framed adds basic support for message framing over streams.

Message are length-prefixed.  The first two bytes are an unsigned 16 bit int
stored in little-endian byte order.  The remaining bytes are the bytes of the
message.

The use of a uint16 means that the maximum possible data length is 65535.

Example:

	package main

	import (
		"github.com/oxtoacart/framed"
		"net"
		"log"
	)

	func main() {
		// Replace host:port with an actual TCP server, for example the echo service
		if conn, err := net.Dial("tcp", "host:port"); err == nil {
			framedConn = Framed{conn}
			if err := framedConn.Write([]byte("Hello World")); err == nil {
				if resp, err := framedConn.Read(); err == nil {
					log.Println("We're done!")
				}
			}
		}
	}
*/
package framed

import (
	"encoding/binary"
	"io"
)

var endianness = binary.LittleEndian

/*
A Framed enhances an io.ReadWriteCloser to provide methods that allow writing
and reading frames.

Although the underlying ReadWriteCloser may be safe to use from multiple
goroutines, a Framed is not.
*/
type Framed struct {
	io.ReadWriteCloser // the raw underlying connection
}

/*
ReadFrame reads the next frame from the Framed.
*/
func (framed Framed) ReadFrame() (frame []byte, err error) {
	var nb uint16
	err = binary.Read(framed, endianness, &nb)
	if err != nil {
		return
	}
	numBytes := int(nb)
	frame = make([]byte, numBytes)
	for totalRead := 0; totalRead < numBytes; {
		var bytesRead int
		bytesRead, err = framed.Read(frame[totalRead:])
		if err != nil {
			return
		}
		totalRead += bytesRead
	}
	return
}

/*
WriteFrame writes all of the supplied bytes to the Framed as a single frame.
*/
func (framed Framed) WriteFrame(byteArrays ...[]byte) (err error) {
	var numBytes uint16
	for _, bytes := range byteArrays {
		numBytes += uint16(len(bytes))
	}
	err = binary.Write(framed, endianness, numBytes)
	for _, bytes := range byteArrays {
		if _, err = framed.Write(bytes); err != nil {
			return
		}
	}
	return
}
