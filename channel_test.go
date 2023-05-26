package pool

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"
)

var (
	network = "tcp"
	address = "127.0.0.1:7777"
	factory = func() (net.Conn, error) { return net.Dial(network, address) }
	maxFree = 3
	maxConn = 5
)

func init() {
	// used for factory function
	go simpleTCPServer()
	time.Sleep(time.Millisecond * 300) // wait until tcp server has been settled

	rand.Seed(time.Now().UTC().UnixNano())
}

func TestNew(t *testing.T) {
	_, err := NewChannelPool(int64(maxFree), int64(maxConn), factory)
	if err != nil {
		t.Errorf("New error: %s", err)
	}
}

func TestChannelPool_Get(t *testing.T) {
	p, _ := NewChannelPool(int64(maxFree), int64(maxConn), factory)
	defer p.Close()

	if p.OpenNum() != maxFree {
		t.Errorf("Get error. Expecting %d, got %d",
			maxFree, p.OpenNum())
	}

	_, err := p.Get()
	if err != nil {
		t.Errorf("Get error: %s", err)
	}

	if p.Len() != (maxFree - 1) {
		t.Errorf("Get error. Expecting %d, got %d",
			(maxFree - 1), p.Len())
	}

	// 拿到所有的free conn
	var wg sync.WaitGroup
	for i := 0; i < (maxFree - 1); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.Get()
			if err != nil {
				t.Errorf("Get error: %s", err)
			}
		}()
	}
	wg.Wait()

	if p.Len() != 0 {
		t.Errorf("Get error. Expecting %d, got %d",
			(maxFree - 1), p.Len())
	}

	_, err = p.Get()
	if err != nil {
		t.Errorf("Get error: %s", err)
	}

	if p.OpenNum() != maxFree+1 {
		t.Errorf("Get error. Expecting %d, got %d",
			maxFree+1, p.OpenNum())
	}
}

func TestChannelPool_Put(t *testing.T) {

	p, _ := NewChannelPool(int64(maxFree), int64(maxConn), factory)
	defer p.Close()

	conns := make([]net.Conn, 0, maxFree)
	for i := 0; i < int(p.maxFree); i++ {
		conn, err := p.Get()
		if err != nil {
			t.Errorf("Get error: %s", err)
		}
		conns = append(conns, conn)
	}

	if p.Len() != 0 {
		t.Errorf("Get error. Expecting %d, got %d",
			0, p.Len())
	}

	for _, conn := range conns {
		err := p.Put(conn)
		if err != nil {
			t.Errorf("Get error: %s", err)
		}
	}

	if p.Len() != maxFree {
		t.Errorf("Get error. Expecting %d, got %d",
			maxFree, p.Len())
	}
}

func TestChannelPool_Close(t *testing.T) {

	p, err := NewChannelPool(int64(maxFree), int64(maxConn), factory)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := p.Get()
	if err != nil {
		t.Errorf("Get error: %s", err)
	}

	err = p.Close()
	if err != nil {
		t.Error(err)
	}

	if p.closed != true {
		t.Errorf("Close error. Expecting %t, got %t",
			true, p.closed)
	}

	if p.Len() != 0 {
		t.Errorf("Close error. Expecting %d, got %d",
			0, p.Len())
	}
	if p.OpenNum() != 1 {
		t.Errorf("Close error. Expecting %d, got %d",
			1, p.OpenNum())
	}

	err = p.Put(conn)
	if err != nil {
		t.Error(err)
	}
	if p.openNum != 0 {
		t.Errorf("Close error. Expecting %d, got %d",
			0, p.openNum)
	}

}

func TestChannelPool_MaxConn(t *testing.T) {
	p, _ := NewChannelPool(int64(maxFree), int64(maxFree), factory)
	defer p.Close()

	// get all free conns
	conns := make([]net.Conn, maxFree)
	for i := 0; i < maxFree; i++ {
		conn, _ := p.Get()
		conns[i] = conn
	}

	if p.Len() != 0 {
		t.Errorf("Get error. Expecting %d, got %d",
			0, p.Len())
	}

	conn := conns[0]

	go func() {
		// 放回conn
		time.Sleep(time.Second)
		if err := p.Put(conn); err != nil {
			t.Error(err)
		}
	}()

	newConn, err := p.Get()
	if err != nil {
		t.Errorf("Get error: %s", err)
	}

	if conn != newConn {
		t.Errorf("Get error. Expecting %v, got %v",
			conn, newConn)
	}

}

func TestPoolWriteRead(t *testing.T) {
	p, _ := NewChannelPool(int64(maxFree), int64(maxFree), factory)
	defer p.Close()

	conn, err := p.Get()
	if err != nil {
		t.Errorf("Get error: %s", err)
	}
	defer conn.Close()

	// write
	_, err = conn.Write([]byte("hello"))
	if err != nil {
		t.Errorf("Get error: %s", err)
	}

	// read
	buffer := make([]byte, 256)
	_, err = conn.Read(buffer)
	if err != nil {
		t.Errorf("Get error: %s", err)
	}
	fmt.Println("client read: ", string(buffer))

}

func simpleTCPServer() {
	l, err := net.Listen(network, address)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}

		go func() {
			buffer := make([]byte, 256)
			conn.Read(buffer)
			fmt.Println("server read: ", string(buffer))
			conn.Write(buffer)
		}()
	}
}
