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
	"github.com/oxtoacart/bpool"
	"io"
)

var endianness = binary.LittleEndian

/*
A Framed enhances an io.ReadWriteCloser to provide methods that allow writing
and reading frames.  It uses buffers from an underlying BytePool.

Although the underlying ReadWriteCloser may be safe to use from multiple
goroutines, a Framed is not.
*/
type Framed struct {
	stream io.ReadWriteCloser // the raw underlying connection
	pool   *bpool.BytePool    // the BytePool used for buffers
}

/*
A Frame is a Frame read from a Framed.
*/
type Frame struct {
	Buffers          [][]byte
	lastBuffer       []byte
	pool             *bpool.BytePool
	length           int
	numBuffers       int
	lastBufferLength int
}

func NewFramed(stream io.ReadWriteCloser, pool *bpool.BytePool) *Framed {
	return &Framed{stream: stream, pool: pool}
}

/*
ReadFrame reads the next frame from the Framed.
*/
func (framed *Framed) ReadFrame() (frame *Frame, err error) {
	var nb uint16
	err = binary.Read(framed.stream, endianness, &nb)
	if err != nil {
		return
	}
	frame = &Frame{
		pool:   framed.pool,
		length: int(nb),
	}
	frame.numBuffers = frame.length/framed.pool.Width() + 1
	frame.lastBufferLength = frame.length % framed.pool.Width()
	frame.Buffers = make([][]byte, frame.numBuffers)
	for i := 0; i < frame.numBuffers; i++ {
		// Set up buffer
		buffer := framed.pool.Get()
		bytesToRead := framed.pool.Width()
		if i == frame.numBuffers-1 {
			// last buffer
			frame.lastBuffer = buffer
			if frame.lastBufferLength != 0 {
				// last buffer is partial
				bytesToRead = frame.lastBufferLength
				buffer = buffer[:bytesToRead]
			}
		}
		frame.Buffers[i] = buffer

		// Read into buffer
		for totalRead := 0; totalRead < bytesToRead; {
			var bytesRead int
			bytesRead, err = framed.stream.Read(buffer[totalRead:])
			if err != nil {
				return
			}
			totalRead += bytesRead
		}
	}
	return
}

/*
WriteFrame writes all of the supplied bytes to the Framed as a single frame.
*/
func (framed *Framed) WriteFrame(byteArrays ...[]byte) (err error) {
	var numBytes uint16
	for _, bytes := range byteArrays {
		numBytes += uint16(len(bytes))
	}
	err = binary.Write(framed.stream, endianness, numBytes)
	for _, bytes := range byteArrays {
		if _, err = framed.stream.Write(bytes); err != nil {
			return
		}
	}
	return
}

func (frame *Frame) Release() {
	for i, buffer := range frame.Buffers {
		if i == frame.numBuffers-1 {
			frame.pool.Put(frame.lastBuffer)
		} else {
			frame.pool.Put(buffer)
		}
	}
}
