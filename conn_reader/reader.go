package conn_reader

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"runtime"
	"sync/atomic"
	"time"

	"../utils"
)

const BUFFER_SIZE = 1280

func init() {
	rand.Seed(time.Now().UnixNano())
}

type ConnReader struct {
	Events   <-chan Event
	EventsIn chan Event
	Count    int32
	pool     *BytesPool
}

const (
	DATA = iota
	EOF
	ERROR
)

type Event struct {
	Type int
	Data []byte
	Obj  interface{}
}

func New() *ConnReader {
	self := new(ConnReader)
	self.EventsIn = make(chan Event)
	self.Events = utils.MakeChan(self.EventsIn).(<-chan Event)
	self.pool = NewPool(BUFFER_SIZE)
	return self
}

func (self *ConnReader) Add(tcpConn *net.TCPConn, obj interface{}) {
	atomic.AddInt32(&self.Count, int32(1))
	go func() {
		defer atomic.AddInt32(&self.Count, int32(-1))
		for {
			buf := self.pool.Get()
			n, err := tcpConn.Read(buf)
			if n > 0 {
				self.EventsIn <- Event{DATA, buf[:n], obj}
			}
			if err != nil {
				if err == io.EOF { //EOF
					self.EventsIn <- Event{EOF, nil, obj}
				} else { //ERROR
					self.EventsIn <- Event{ERROR, []byte(err.Error()), obj}
				}
				return
			}
		}
	}()
}

func (self *ConnReader) Close() {
	close(self.EventsIn)
}

type BytesPool struct {
	pool  chan []byte
	size  int
	reuse int
	alloc int
}

func NewPool(size int) *BytesPool {
	return &BytesPool{
		size: size,
		pool: make(chan []byte, 10240),
	}
}

func (self *BytesPool) Get() []byte {
	var bs []byte
	select {
	case bs = <-self.pool:
		self.reuse++
	default:
		self.alloc++
		bs = make([]byte, self.size)
	}
	fmt.Printf("%d reused, %d alloc\n", self.reuse, self.alloc)
	runtime.SetFinalizer(&bs, func(bs *[]byte) {
		select {
		case self.pool <- *bs:
		default:
		}
	})
	return bs
}
