package main

import (
	"flag"
	"fmt"
	"github.com/oxtoacart/framed"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
)

const (
	serverAddr = "127.0.0.1:10081"
)

var (
	mode = flag.String("mode", "server", "Mode (server or client)")
	wg   sync.WaitGroup
)

func main() {
	runtime.GOMAXPROCS(1)
	flag.Parse()
	file, err := os.Create(fmt.Sprintf("/tmp/framed_profile_%s", *mode))
	if err != nil {
		log.Fatal("Unable to create CPU profile file: %s", err)
	}
	pprof.StartCPUProfile(file)
	defer pprof.StopCPUProfile()
	if *mode == "client" {
		client()
	} else {
		server()
	}
}

func server() {
	log.Printf("Starting server at %s", serverAddr)
	if listener, err := net.Listen("tcp", serverAddr); err != nil {
		log.Fatalf("Unable to listen: %s", err)
	} else {
		for {
			if conn, err := listener.Accept(); err != nil {
				log.Printf("Unable to accept: %s", err)
			} else {
				f := framed.NewFramed(conn)
				go func() {
					if frame, err := f.ReadInitial(); err != nil {
						log.Printf("Unable to read initial frame: %s", frame)
					} else {
						for {
							if err := frame.CopyTo(conn); err != nil {
								pprof.StopCPUProfile()
								log.Fatalf("Unable to copy: %s", err)
							} else {
								if frame, err = frame.Next(); err != nil {
									log.Fatalf("Unable to read next frame")
								}
							}
						}
					}
				}()
			}
		}
	}
}

func client() {
	for i := 0; i < 1; i++ {
		wg.Add(1)
		go doClient()
	}
	wg.Wait()
}

func doClient() {
	log.Printf("Starting client connection to server at %s", serverAddr)
	header := []byte{}
	data := []byte("Hell World")
	if conn, err := net.Dial("tcp", serverAddr); err != nil {
		log.Fatalf("Unable to dial server: %s", err)
	} else {
		f := framed.NewFramed(conn)
		// Read
		go io.Copy(ioutil.Discard, conn)

		// Write
		for {
			if err := f.WriteFrame(header, data); err != nil {
				log.Fatalf("Unable to write frame: %s", err)
			}
		}
	}
	wg.Done()
}
