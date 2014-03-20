/*
Package framed adds basic support for message framing over streams.

Messages contain an header and a body, both of which are length prefixed.

Here are the bytes (stored in little-endian byte order):

0-2: unsigned 16 bit int header length
2-4: unsinged 16 bit int body length
4+:  message content (header and body)

The use of a uint16 means that the maximum possible header and body lengths
are 65535 each.

Important Note for using with TCP -

When using framed with a TCPConn, we recommend calling
TCPConn.SetNoDelay(false).

framed was designed to maximize the chance of zero-copy ops being used to send
frames from one connection to another. Because of this, it writes a lot of
small pieces of data to the underlying stream (e.g. the length headers).  If
TCP is being used without Nagle's algorithm (i.e. setting
TCPConn.SetNoDelay(true), which is the default), then it can result in fairly
extreme packet fragmentation and consequently ballooning overhead.

Example Usage:

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
							nextFrame, err := frame.Next()
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
	raw            io.ReadWriteCloser // the raw underlying connection
	hasReadInitial bool
}

// AlreadyReadError is returned when someone has already read from a Frame
type AlreadyReadError string

func (err AlreadyReadError) Error() string {
	return string(err)
}

// Frame encapsulates a frame from a Framed
type Frame struct {
	framed         *Framed
	headerLength   uint16
	bodyLength     uint16
	bytesRemaining int
	header         *frameSection
	body           *frameSection
	doneReading    chan bool
}

// frameSection encapsulates a section of a frame (header or body)
type frameSection struct {
	frame          *Frame
	init           func() error
	bytesRemaining int
	startedReading bool
}

// NewFramed creates a new Framed on top of the given readWriteCloser
func NewFramed(readWriteCloser io.ReadWriteCloser) *Framed {
	return &Framed{readWriteCloser, false}
}

// Close() implements method Close() from io.Closer.
func (framed *Framed) Close() error {
	return framed.raw.Close()
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
Next returns the next frame from the Framed underlying the frame on which
it is called.  This method blocks until the previous frame has been consumed.
If there are no more frames, it returns io.EOF as an error.
*/
func (frame *Frame) Next() (nextFrame *Frame, err error) {
	<-frame.doneReading
	return frame.framed.nextFrame()
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
		if _, err = framed.raw.Write(header); err != nil {
			return err
		}
	}

	if body != nil {
		_, err = framed.raw.Write(body)
	}

	return
}

// WriteHeader writes a frame header with the given lengths to the Framed.
func (framed *Framed) WriteHeader(headerLength uint16, bodyLength uint16) (err error) {
	return writeHeaderTo(framed.raw, headerLength, bodyLength)
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
	var n int64
	n, err = io.CopyN(out, frame.framed.raw, int64(frame.headerLength+frame.bodyLength))
	frame.bytesRemaining -= int(n)
	frame.checkDone()
	return
}

/*
Read implements io.Reader.Read for a frameSection.  Just like the usual Read(),
this one may read incompletely, so make sure to call it until it returns EOF.
*/
func (section *frameSection) Read(p []byte) (n int, err error) {
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
	n, err = section.frame.framed.raw.Read(p)
	nint := int(n)
	section.frame.bytesRemaining -= nint
	section.bytesRemaining -= nint
	if section.bytesRemaining == 0 {
		err = io.EOF
	}
	section.frame.checkDone()
	return
}

func (frame *Frame) Discard() (err error) {
	var n int64
	n, err = io.CopyN(ioutil.Discard, frame.framed.raw, int64(frame.header.bytesRemaining+frame.body.bytesRemaining))
	frame.bytesRemaining -= int(n)
	frame.checkDone()
	return
}

func (section *frameSection) discard() (err error) {
	if section.init != nil {
		section.init()
	}
	if section.bytesRemaining > 0 {
		var n int64
		n, err = io.Copy(ioutil.Discard, section)
		nint := int(n)
		section.frame.bytesRemaining -= nint
		section.bytesRemaining -= nint
	}
	section.frame.checkDone()
	return
}

func (frame *Frame) HeaderLength() uint16 {
	return frame.headerLength
}

func (frame *Frame) BodyLength() uint16 {
	return frame.bodyLength
}

func (frame *Frame) Header() io.Reader {
	return frame.header
}

func (frame *Frame) Body() io.Reader {
	return frame.body
}

func (framed *Framed) nextFrame() (frame *Frame, err error) {
	frame = &Frame{framed: framed, doneReading: make(chan bool, 1)}
	frame.header = &frameSection{frame: frame}
	frame.body = &frameSection{frame: frame, init: frame.header.discard}
	if err = frame.readLengths(); err != nil {
		return
	}
	return
}

func (frame *Frame) readLengths() (err error) {
	if err = binary.Read(frame.framed.raw, endianness, &frame.headerLength); err != nil {
		return
	}
	if err = binary.Read(frame.framed.raw, endianness, &frame.bodyLength); err != nil {
		return
	}
	frame.header.bytesRemaining = int(frame.headerLength)
	frame.body.bytesRemaining = int(frame.bodyLength)
	frame.bytesRemaining = frame.header.bytesRemaining + frame.body.bytesRemaining
	return
}

func (frame *Frame) checkDone() {
	if frame.bytesRemaining == 0 {
		frame.doneReading <- true
	}
}

func writeHeaderTo(out io.Writer, headerLength uint16, bodyLength uint16) (err error) {
	if err = binary.Write(out, endianness, headerLength); err != nil {
		return
	}
	err = binary.Write(out, endianness, bodyLength)
	return
}
