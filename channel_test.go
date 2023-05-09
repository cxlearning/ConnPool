package pool

import (
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

func TestPool_Get_Impl(t *testing.T) {
	p, _ := NewChannelPool(int64(maxFree), int64(maxConn), factory)
	defer p.Close()
	conn, err := p.Get()
	if err != nil {
		t.Errorf("Get error: %s", err)
	}

	_, ok := conn.(*PoolConn)
	if !ok {
		t.Errorf("Conn is not of type poolConn")
	}
}

func TestPool_Get(t *testing.T) {
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

func TestPool_Put(t *testing.T) {
	p, err := NewChannelPool(int64(maxFree), int64(maxConn), factory)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	// get/create from the pool
	conns := make([]net.Conn, maxFree)
	for i := 0; i < maxFree; i++ {
		conn, _ := p.Get()
		conns[i] = conn
	}

	// now put them all back
	for _, conn := range conns {
		if err = p.Put(conn); err != nil {
			t.Error(err)
		}
	}

	if p.Len() != maxFree {
		t.Errorf("Put error len. Expecting %d, got %d",
			maxFree, p.Len())
	}

	conn, _ := p.Get()
	p.Close() // close pool

	if p.OpenNum() != 1 {
		t.Errorf("Get error. Expecting %d, got %d",
			1, p.OpenNum())
	}

	if p.Len() != 0 {
		t.Errorf("the num of free conn is not zero")
	}

	err = p.Put(conn) // 向close的pool放入conn
	if err != nil {
		t.Error(err)
	}

	if p.OpenNum() != 0 {
		t.Errorf("Get error. Expecting %d, got %d",
			0, p.OpenNum())
	}

}

func TestClose(t *testing.T) {

	p, err := NewChannelPool(int64(maxFree), int64(maxConn), factory)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := p.Get()
	if err != nil {
		t.Errorf("Get error: %s", err)
	}
	
	p.Close()

	cp := p.(*channelPool)

	if cp.closed != true {
		t.Errorf("Close error. Expecting %t, got %t",
			true, cp.closed)
	}

	if cp.Len() != 0 {
		t.Errorf("Close error. Expecting %d, got %d",
			0, cp.Len())
	}
	if cp.OpenNum() != 1 {
		t.Errorf("Close error. Expecting %d, got %d",
			1, cp.OpenNum())
	}

	err = cp.Put(conn)
	if err != nil {
		t.Error(err)
	}
	if cp.openNum != 0 {
		t.Errorf("Close error. Expecting %d, got %d",
			0, cp.openNum)
	}

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
		}()
	}
}
