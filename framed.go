/*
Package framed adds basic support for message framing over streams.

Messages contain an header and a body, both of which are length prefixed.

Here are the bytes (stored in little-endian byte order):

0-2: unsigned 16 bit int header length
2-4: unsinged 16 bit int body length
4+:  message content (header and body)

The use of a uint16 means that the maximum possible header and body lengths
are 65535 each.

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
	"io/ioutil"
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
	hasReadInitial     bool
}

type AlreadyReadError string

func (err AlreadyReadError) Error() string {
	return string(err)
}

type BufferTooSmall string

func (err BufferTooSmall) Error() string {
	return string(err)
}

type Frame struct {
	framed        *Framed
	headerLength  int16
	bodyLength    int16
	hasReadHeader bool
	hasReadBody   bool
}

func NewFramed(readWriteCloser io.ReadWriteCloser) *Framed {
	return &Framed{readWriteCloser, false}
}

/*
ReadInitial reads the initial frame from the framed
*/
func (framed *Framed) ReadInitial() (frame *Frame, err error) {
	if framed.hasReadInitial {
		return nil, AlreadyReadError("Initial Frame already read")
	}
	frame, err = framed.nextFrame()
	framed.hasReadInitial = true
	return
}

func (frame *Frame) NextFrame() (nextFrame *Frame, err error) {
	if !frame.hasReadBody {
		frame.CopyBody(ioutil.Discard)
	}
	nextFrame, err = frame.framed.nextFrame()
	return
}

func (frame *Frame) CopyHeader(out io.Writer) (err error) {
	if frame.hasReadHeader {
		return AlreadyReadError("Header Already Read")
	}
	_, err = io.CopyN(out, frame.framed, int64(frame.headerLength))
	frame.hasReadHeader = true
	return
}

func (frame *Frame) CopyBody(out io.Writer) (err error) {
	if !frame.hasReadHeader {
		if err := frame.CopyHeader(ioutil.Discard); err != nil {
			return err
		}
	}
	if frame.hasReadBody {
		err = AlreadyReadError("Body Already Read")
		return
	}
	_, err = io.CopyN(out, frame.framed, int64(frame.bodyLength))
	frame.hasReadBody = true
	return
}

func (frame *Frame) ReadHeader(buffer []byte) (n int, err error) {
	if len(buffer) < int(frame.headerLength) {
		return 0, BufferTooSmall(fmt.Sprintf("Buffer too small. %d bytes required for header.", frame.headerLength))
	}
	if frame.hasReadHeader {
		return 0, AlreadyReadError("Header Already Read")
	}
	frame.framed.Read(buffer[0:frame.headerLength])
	frame.hasReadHeader = true
	return
}

func (frame *Frame) ReadBody(buffer []byte) (n int, err error) {
	if len(buffer) < int(frame.bodyLength) {
		return 0, BufferTooSmall(fmt.Sprintf("Buffer too small. %d bytes required for body.", frame.bodyLength))
	}
	if !frame.hasReadHeader {
		if err := frame.CopyHeader(ioutil.Discard); err != nil {
			return 0, err
		}
	}
	if frame.hasReadBody {
		return 0, AlreadyReadError("Body Already Read")
	}
	frame.framed.Read(buffer[0:frame.bodyLength])
	frame.hasReadBody = true
	return
}

func (framed *Framed) WriteFrame(header []byte, body []byte) (err error) {
	if err = framed.WriteHeader(int16(len(header)), int16(len(body))); err != nil {
		return err
	}
	if _, err = framed.Write(header); err != nil {
		return err
	}
	_, err = framed.Write(body)
	return
}

func (framed *Framed) WriteHeader(headerLength int16, bodyLength int16) (err error) {
	return writeHeaderTo(framed, headerLength, bodyLength)
}

func (frame *Frame) CopyTo(out io.Writer) (err error) {
	if err = writeHeaderTo(out, frame.headerLength, frame.bodyLength); err != nil {
		return
	}
	if err = frame.CopyHeader(out); err != nil {
		return
	}
	err = frame.CopyBody(out)
	return
}

func (frame *Frame) readLengths() (err error) {
	if err = binary.Read(frame.framed, endianness, &frame.headerLength); err != nil {
		return
	}
	err = binary.Read(frame.framed, endianness, &frame.bodyLength)
	return
}

func writeHeaderTo(out io.Writer, headerLength int16, bodyLength int16) (err error) {
	if err = binary.Write(out, endianness, headerLength); err != nil {
		return
	}
	err = binary.Write(out, endianness, bodyLength)
	return
}

func (framed *Framed) nextFrame() (frame *Frame, err error) {
	frame = &Frame{framed: framed}
	err = frame.readLengths()
	return
}
