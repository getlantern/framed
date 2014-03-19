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
		"io/ioutil"
		"log"
		"net"
	)

	func main() {
		// Replace host:port with an actual TCP server, for example the echo service
		if conn, err := net.Dial("tcp", "host:port"); err == nil {
			framedConn = Framed{conn}
			if err := framedConn.WriteFrame([]byte("Header"), []byte("Hello World")); err == nil {
				if err, frame := framedConn.ReadInitial(); err == nil {
					// Note - Read is just like io.Reader.Read(), so we use ioutil.ReadAll
					if header, err := ioutil.ReadAll(framedConn.Header); err == nil {
						if body, err := ioutil.ReadAll(framedConn.Body); err == nil {
							nextFrame, err := frame.NextFrame()
							// And so on
						}
					}
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

// AlreadyReadError is returned when someone has already read from a Frame
type AlreadyReadError string

func (err AlreadyReadError) Error() string {
	return string(err)
}

// Frame encapsulates a frame from a Framed
type Frame struct {
	framed       *Framed
	headerLength uint16
	bodyLength   uint16
	header       *FrameSection
	body         *FrameSection
}

// FrameSection encapsulates a section of a frame (header or body)
type FrameSection struct {
	frame          *Frame
	init           func() error
	bytesRemaining int
	startedReading bool
}

// NewFramed creates a new Framed on top of the given readWriteCloser
func NewFramed(readWriteCloser io.ReadWriteCloser) *Framed {
	return &Framed{readWriteCloser, false}
}

/*
ReadInitial reads the initial frame from the framed.  It returns an
AlreadyReadError if the initial frame has already been read previously.
*/
func (framed *Framed) ReadInitial() (frame *Frame, err error) {
	if framed.hasReadInitial {
		return nil, AlreadyReadError("Initial Frame already read")
	}
	frame, err = framed.nextFrame()
	framed.hasReadInitial = true
	return
}

/*
NextFrame returns the next frame from the Framed underlying the frame on which
it is called.
*/
func (frame *Frame) NextFrame() (nextFrame *Frame, err error) {
	if err = frame.header.drain(); err != nil {
		return
	}
	if err = frame.body.drain(); err != nil {
		return
	}
	nextFrame, err = frame.framed.nextFrame()
	return
}

/*
WriteFrame writes the given header and body to the Framed.
Either or both can be nil.
*/
func (framed *Framed) WriteFrame(header []byte, body []byte) (err error) {
	headerLength := 0
	bodyLength := 0
	if header != nil {
		headerLength = len(header)
	}
	if body != nil {
		bodyLength = len(body)
	}

	if err = framed.WriteHeader(uint16(headerLength), uint16(bodyLength)); err != nil {
		return err
	}

	if header != nil {
		if _, err = framed.Write(header); err != nil {
			return err
		}
	}

	if body != nil {
		_, err = framed.Write(body)
	}

	return
}

// WriteHeader writes a frame header with the given lengths to the Framed.
func (framed *Framed) WriteHeader(headerLength uint16, bodyLength uint16) (err error) {
	return writeHeaderTo(framed, headerLength, bodyLength)
}

/*
CopyTo copies the given Frame to the given Writer.  If reading of the Frame
has already started before CopyTo is called, CopyTo returns an
AlreadyReadError.
*/
func (frame *Frame) CopyTo(out io.Writer) (err error) {
	if frame.header.startedReading || frame.body.startedReading {
		return AlreadyReadError("Already read from frame, cannot copy")
	}
	if err = writeHeaderTo(out, frame.headerLength, frame.bodyLength); err != nil {
		return
	}
	_, err = io.CopyN(out, frame.framed, int64(frame.headerLength+frame.bodyLength))
	return
}

/*
Read implements io.Reader.Read for a FrameSection.  Just like the usual Read(),
this one may read incompletely, so make sure to call it until it returns EOF.
*/
func (section *FrameSection) Read(p []byte) (n int, err error) {
	if section.bytesRemaining == 0 {
		return 0, err
	}
	if section.init != nil {
		if err = section.init(); err != nil {
			return 0, err
		}
	}
	section.startedReading = true
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

func (frame *Frame) HeaderLength() int {
	return int(frame.headerLength)
}

func (frame *Frame) BodyLength() int {
	return int(frame.bodyLength)
}

func (frame *Frame) Header() io.Reader {
	return frame.header
}

func (frame *Frame) Body() io.Reader {
	return frame.body
}

func (framed *Framed) nextFrame() (frame *Frame, err error) {
	frame = &Frame{framed: framed}
	frame.header = &FrameSection{frame: frame}
	frame.body = &FrameSection{frame: frame, init: frame.header.drain}
	if err = frame.readLengths(); err != nil {
		return
	}
	return
}

func (frame *Frame) readLengths() (err error) {
	if err = binary.Read(frame.framed, endianness, &frame.headerLength); err != nil {
		return
	}
	if err = binary.Read(frame.framed, endianness, &frame.bodyLength); err != nil {
		return
	}
	frame.header.bytesRemaining = int(frame.headerLength)
	frame.body.bytesRemaining = int(frame.bodyLength)
	return
}

func (section *FrameSection) drain() (err error) {
	if section.bytesRemaining > 0 {
		_, err = io.Copy(ioutil.Discard, section)
	}
	return
}

func writeHeaderTo(out io.Writer, headerLength uint16, bodyLength uint16) (err error) {
	if err = binary.Write(out, endianness, headerLength); err != nil {
		return
	}
	err = binary.Write(out, endianness, bodyLength)
	return
}
