package pool

import (
	"errors"
	"net"
)

var (
	ErrClosed = errors.New("pool is closed")
)

type Pool interface {
	//Get 获得conn
	Get() (net.Conn, error)

	Put(net.Conn) error

	//Close 关闭管理的所有conn，pool不可用
	Close() error
}
