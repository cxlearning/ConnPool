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

	//Put 归还conn
	Put(conn net.Conn) error

	//Close 关闭管理的所有conn，pool不可用
	Close()

	// Len pool中存储的conn数量
	Len() int

	// OpenNum pool中已创建的conn数量
	OpenNum() int
}
