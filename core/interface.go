package core

import (
	"net"
)

type IPoll interface {
	Add(conn net.Conn) error
	Remove(conn net.Conn) error
	Wait(int64) ([]net.Conn, error)
}
