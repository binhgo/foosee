package poll

import (
	"net"
	"sync"
	"syscall"
)

type EPoll struct {
	fd          int
	changes     []syscall.Kevent_t
	events      []syscall.Kevent_t
	connections map[uint64]net.Conn
	lock        *sync.RWMutex
}

func NewEPoll() *EPoll {
	fd, err := syscall.Kqueue()
	if err != nil {
		return nil
	}

	return &EPoll{
		fd:          fd,
		events:      make([]syscall.Kevent_t, 64),
		lock:        &sync.RWMutex{},
		connections: make(map[uint64]net.Conn),
	}
}

func (k *EPoll) Add(wsConn net.Conn) error {

	return nil
}

func (k *EPoll) Remove(wsConn net.Conn) error {

	return nil
}

func (k *EPoll) Wait(timeout int64) ([]net.Conn, error) {

	return nil, nil
}
