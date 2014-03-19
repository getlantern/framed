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

type FrameSection struct {
	frame          *Frame
	init           func() error
	bytesRemaining int
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
		if body, err := frame.Body(); err != nil {
			return nil, err
		} else if _, err = io.Copy(ioutil.Discard, body); err != nil {
			return nil, err
		}
	}
	nextFrame, err = frame.framed.nextFrame()
	return
}

func (frame *Frame) Header() (header *FrameSection, err error) {
	if frame.hasReadHeader {
		return nil, AlreadyReadError("Header Already Read")
	}
	header = &FrameSection{frame: frame, bytesRemaining: int(frame.headerLength)}
	frame.hasReadHeader = true
	return
}

func (frame *Frame) Body() (body *FrameSection, err error) {
	if frame.hasReadBody {
		return nil, AlreadyReadError("Body Already Read")
	}
	body = &FrameSection{frame: frame, bytesRemaining: int(frame.bodyLength), init: frame.readHeaderIfNecessary}
	frame.hasReadBody = true
	return
}

func (section *FrameSection) Read(p []byte) (n int, err error) {
	if section.init != nil {
		if err = section.init(); err != nil {
			return 0, err
		}
	}
	if section.bytesRemaining == 0 {
		return 0, err
	}
	if len(p) > section.bytesRemaining {
		p = p[0:section.bytesRemaining]
	}
	n, err = section.frame.framed.Read(p)
	if n > 0 {
		section.bytesRemaining -= n
	}
	if section.bytesRemaining == 0 {
		err = io.EOF
	}
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
	if frame.hasReadHeader || frame.hasReadBody {
		return AlreadyReadError("Already read from frame, cannot copy")
	}
	if err = writeHeaderTo(out, frame.headerLength, frame.bodyLength); err != nil {
		return
	}
	_, err = io.CopyN(out, frame.framed, int64(frame.headerLength+frame.bodyLength))
	return
}

func (frame *Frame) readLengths() (err error) {
	if err = binary.Read(frame.framed, endianness, &frame.headerLength); err != nil {
		return
	}
	err = binary.Read(frame.framed, endianness, &frame.bodyLength)
	return
}

func (frame *Frame) readHeaderIfNecessary() error {
	if !frame.hasReadHeader {
		if header, err := frame.Header(); err != nil {
			return err
		} else if _, err = io.Copy(ioutil.Discard, header); err != nil {
			return err
		}
	}
	return nil
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
