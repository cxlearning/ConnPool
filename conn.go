package pool

import (
	"net"
)

// PoolConn 整个生命周期由pool管理
type PoolConn struct {
	net.Conn
	poll *channelPool
}
