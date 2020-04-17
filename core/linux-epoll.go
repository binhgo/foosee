package core

//
// import (
// 	"fmt"
// 	"net"
// 	"sync"
// 	"syscall"
// 	"time"
//
// 	"github.com/binhgo/foosee/util"
// )
//
// type EPoll struct {
// 	fd          int
// 	events      []syscall.EpollEvent
// 	connections map[uint64]net.Conn
// 	lock        *sync.RWMutex
// }
//
// func NewEPoll() *EPoll {
//
// 	fd, err := syscall.EpollCreate1(0)
// 	if err != nil {
// 		panic(err)
// 	}
//
// 	return &EPoll{
// 		fd:          fd,
// 		events:      make([]syscall.EpollEvent, 64),
// 		lock:        &sync.RWMutex{},
// 		connections: make(map[uint64]net.Conn),
// 	}
// }
//
// func (k *EPoll) Add(wsConn net.Conn) error {
//
// 	fmt.Println("add connection")
//
// 	k.lock.Lock()
// 	defer k.lock.Unlock()
//
// 	wsfd := util.GetFD2(wsConn)
//
// 	if wsfd == 0 {
// 		return nil
// 	}
//
// 	err := syscall.EpollCtl(k.fd, syscall.EPOLL_CTL_ADD, int(wsfd),
// 		&syscall.EpollEvent{Fd: int32(wsfd),
// 			Events: syscall.EPOLLIN,
// 		},
// 	)
// 	if err != nil {
// 		panic(err)
// 	}
//
// 	k.connections[wsfd] = wsConn
//
// 	fmt.Println("add connection end")
//
// 	return nil
// }
//
// func (k *EPoll) Remove(wsConn net.Conn) error {
// 	fmt.Println("remove connection")
//
// 	k.lock.Lock()
// 	defer k.lock.Unlock()
//
// 	wsfd := util.GetFD2(wsConn)
//
// 	if wsfd == 0 {
// 		return nil
// 	}
//
// 	err := syscall.EpollCtl(k.fd, syscall.EPOLL_CTL_MOD, int(wsfd),
// 		&syscall.EpollEvent{Fd: int32(wsfd),
// 			Events: syscall.EPOLLIN,
// 		},
// 	)
// 	if err != nil {
// 		panic(err)
// 	}
//
// 	delete(k.connections, wsfd)
//
// 	fmt.Println("remove connection end")
//
// 	return nil
// }
//
// func (k *EPoll) Wait(timeout int64) ([]net.Conn, error) {
// 	var conns []net.Conn
//
// 	var nev int
// 	var err error
//
// 	if timeout >= 0 {
// 		nev, err = syscall.EpollWait(k.fd, k.events, int(time.Duration(timeout)/time.Millisecond))
// 	} else {
// 		nev, err = syscall.EpollWait(k.fd, k.events, -1)
// 	}
//
// 	if err != nil && err != syscall.EINTR {
// 		panic(err)
// 	}
//
// 	k.lock.RLock()
// 	for i := 0; i < nev; i++ {
// 		conn := k.connections[uint64(k.events[i].Fd)]
// 		conns = append(conns, conn)
// 	}
// 	k.lock.RUnlock()
//
// 	return conns, nil
// }
