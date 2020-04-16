package poll

import (
	"fmt"
	"net"
	"sync"
	"syscall"

	"github.com/binhgo/foosee/util"
)

var timespec = &syscall.Timespec{
	Sec:  0,
	Nsec: 100000,
}

type KQueue struct {
	fd          int
	changes     []syscall.Kevent_t
	events      []syscall.Kevent_t
	connections map[uint64]net.Conn
	lock        *sync.RWMutex
}

func NewKQueue() *KQueue {
	fd, err := syscall.Kqueue()
	if err != nil {
		return nil
	}

	return &KQueue{
		fd:          fd,
		events:      make([]syscall.Kevent_t, 64),
		lock:        &sync.RWMutex{},
		connections: make(map[uint64]net.Conn),
	}
}

func (k *KQueue) Add(wsConn net.Conn) error {

	fmt.Println("add connection")

	k.lock.Lock()
	defer k.lock.Unlock()

	fd := util.GetFD2(wsConn)

	if fd == 0 {
		return nil
	}

	readChange := syscall.Kevent_t{
		Ident:  fd,
		Flags:  syscall.EV_ADD,
		Filter: syscall.EVFILT_READ,
	}

	k.changes = append(k.changes, readChange)

	k.connections[fd] = wsConn

	fmt.Println("add connection end")

	return nil
}

func (k *KQueue) Remove(wsConn net.Conn) error {

	fmt.Println("remove connection")

	k.lock.Lock()
	defer k.lock.Unlock()

	fd := util.GetFD2(wsConn)

	if fd == 0 {
		return nil
	}

	deletedChange := syscall.Kevent_t{
		Ident: fd,
		Flags: syscall.EV_DELETE,
	}

	k.changes = append(k.changes, deletedChange)

	delete(k.connections, fd)

	fmt.Println("remove connection end")

	return nil
}

func (k *KQueue) Wait(timeout int64) ([]net.Conn, error) {

	var conns []net.Conn

	var nev int
	var err error
	if timeout >= 0 {
		var ts syscall.Timespec
		ts.Nsec = timeout
		nev, err = syscall.Kevent(k.fd, k.changes, k.events, &ts)

	} else {
		nev, err = syscall.Kevent(k.fd, k.changes, k.events, timespec)
	}

	if err != nil && err != syscall.EINTR {
		panic(err)
	}

	k.lock.RLock()
	for i := 0; i < nev; i++ {
		conn := k.connections[k.events[i].Ident]
		conns = append(conns, conn)
	}
	k.lock.RUnlock()

	return conns, nil
}
