package pool

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
)

type channelPool struct {

	//保证并发安全(connCh的修改)
	mu sync.RWMutex

	//存储未使用的conn
	connCh chan net.Conn

	closed bool // pool是否已关闭

	// net.Conn 生产者
	factory Factory

	maxConn int64 // 最大conn数量, <= 0 不限制

	openNum int64 // 已创建连接数
}

var (
	ErrTimeOut = errors.New("time out")
)

// Factory net.Conn 生产者
type Factory func() (net.Conn, error)

func NewChannelPool(maxFree, maxConn int64, factory Factory) (Pool, error) {

	if maxFree <= 0 || maxConn < 0 || maxFree > maxConn {
		return nil, errors.New("invalid capacity settings")
	}

	p := &channelPool{
		connCh:  make(chan net.Conn, maxFree),
		factory: factory,
	}

	// 初始化链接
	for i := 0; i < int(maxFree); i++ {
		conn, err := factory()
		if err != nil {
			p.Close()
			return nil, fmt.Errorf("factory is not able to fill the pool: %s", err)
		}
		p.connCh <- conn
	}
	p.openNum = maxFree
	return p, nil
}

//func (p *channelPool) getConnCh() chan net.Conn {
//	p.mu.RLock()
//	connCh := p.connCh
//	p.mu.RUnlock()
//	return connCh
//}

func (p *channelPool) Get() (net.Conn, error) {
	return p.GetWitchContext(context.Background())
}

func (p *channelPool) GetWitchContext(ctx context.Context) (net.Conn, error) {

	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()
		return nil, ErrClosed
	}

	// 有空闲链接, 或者已达到最大链接数，都只能从connCh中获取
	if len(p.connCh) > 0 || (p.maxConn > 0 && p.openNum >= p.maxConn) {
		p.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ErrTimeOut
		case conn := <-p.connCh:
			if conn == nil {
				return nil, ErrClosed
			}
			return p.wrapConn(conn), nil
		}
	}

	defer p.mu.Unlock()
	// 未达到最大链接数，可以创建新链接
	conn, err := p.factory()
	if err != nil {
		return nil, err
	}
	p.openNum++
	return p.wrapConn(conn), nil
}

func (p *channelPool) Put(conn net.Conn) error {
	return p.PutWithContext(context.Background(), conn)
}

func (p *channelPool) PutWithContext(ctx context.Context, conn net.Conn) error {

	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 已关闭
	if p.closed {
		err := conn.Close()
		if err == nil {
			p.openNum--
		}
		return err
	}

	select {
	case <-ctx.Done():
		return ErrTimeOut
	case p.connCh <- conn:
		return nil
	}
}

func (p *channelPool) Close() {

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.closed = true
	close(p.connCh)
	for c := range p.connCh {
		_ = c.Close()
		p.openNum--
	}
}

func (p *channelPool) Len() int {
	return len(p.connCh)
}

func (p *channelPool) OpenNum() int {
	return int(p.openNum)
}

func (p *channelPool) wrapConn(conn net.Conn) net.Conn {
	pc := &PoolConn{Conn: conn}
	return pc
}
