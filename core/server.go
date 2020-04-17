package core

import (
	"bytes"
	"fmt"
	"log"
	"net"

	"github.com/binhgo/foosee/util"
	"github.com/gobwas/ws/wsutil"
)

type HandleFunc func(request Request) Response

type Server struct {
	Poll        IPoll
	PollTimeout int64
	handlers    map[string]HandleFunc
}

func NewServer(kqTimeout int64) *Server {
	kq := NewKQueue()
	return &Server{
		Poll:        kq,
		PollTimeout: kqTimeout,
		handlers:    make(map[string]HandleFunc),
	}
}

func (s *Server) Start() {

	for {
		conns, err := s.Poll.Wait(s.PollTimeout)
		if err != nil {
			fmt.Printf("Failed to epoll wait %v", err)
			continue
		}

		for _, conn := range conns {
			if conn == nil {
				break
			}

			s.Process(conn)
		}
	}
}

func (s *Server) Process(conn net.Conn) {

	msg, _, err := wsutil.ReadClientData(conn)
	if err != nil {
		if err := s.Poll.Remove(conn); err != nil {
			log.Printf("Failed to remove %v", err)
		}
		conn.Close()
	}

	fmt.Println(string(msg))

	req := Request{}
	err = util.FromJson(msg, &req)

	bb := bytes.Buffer{}

	if err != nil {
		bb.WriteString("ERROR PARSE REQUEST: [")
		bb.WriteString(string(msg))
		bb.WriteString("]")

		wsutil.WriteServerText(conn, bb.Bytes())
		return
	}

	// process data
	handleFunc := s.GetHandler(req.Action)
	if handleFunc == nil {
		bb.WriteString("NO HANDLER for METHOD: [")
		bb.WriteString(req.Action)
		bb.WriteString("]")

		wsutil.WriteServerText(conn, bb.Bytes())
		return
	}

	response := handleFunc(req)

	jsn, err := util.ToJson(response)
	if err != nil {
		panic(err)
	}

	wsutil.WriteServerText(conn, jsn)
}

func (s *Server) SetHandle(path string, handler HandleFunc) {
	s.handlers[path] = handler
}

func (s *Server) GetHandler(path string) HandleFunc {
	return s.handlers[path]
}
