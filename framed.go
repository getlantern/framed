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
	"fmt"
	"io"
	"bytes"
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
	var numBytes uint16
	err = binary.Read(framed, endianness, &numBytes)
	if err != nil {
		return
	}
	frame = make([]byte, numBytes)
	bytesRead, err := framed.Read(frame)
	if err != nil {
		return
	}
	if bytesRead < int(numBytes) {
		err = fmt.Errorf("Too few bytes read.  Expected %s, got %s", numBytes, bytesRead)
	}
	return
}

/*
WriteFrame writes all of the supplied bytes to the Framed as a single frame.
*/
func (framed Framed) WriteFrame(byteArrays ...[]byte) (err error) {
	var numBytes uint16
	for _, bytes := range(byteArrays) {
		numBytes += uint16(len(bytes))
	}
	err = binary.Write(framed, endianness, numBytes)
	buf := bytes.NewBuffer(make([]byte, 0))
	for _, b := range(byteArrays) {
		buf.Write(b) 
	}
	framed.Write(buf.Bytes())
	// TODO: figure out why the below doesn't work reliably with ftcp, as it
	// might be a little more efficient if we can get it to work
//	for _, bytes := range(byteArrays) {
//		framed.Write(bytes)
//	}
	return
}
