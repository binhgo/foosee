package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strconv"

	"github.com/binhgo/foosee/server"
	"github.com/gobwas/ws"
)

var srv *server.Server

func main() {

	srv = server.NewServer(-10)
	go srv.Start()

	srv.SetHandle("GET-ORDER", handleOrder)

	http.HandleFunc("/", UpgradeWebsocket)
	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func handleOrder(request server.Request) server.Response {

	by, err := json.Marshal(request.Data)
	if err != nil {
		return server.Response{
			Status:  "ERROR",
			Message: err.Error(),
		}
	}

	var data *PingMsg
	err = json.Unmarshal(by, &data)
	if err != nil {
		return server.Response{
			Status:  "ERROR",
			Message: err.Error(),
		}
	}

	md := data.Name
	md = md + "-" + strconv.Itoa(rand.Intn(100000))

	return server.Response{
		Status:  "OK",
		Message: md,
	}
}

func UpgradeWebsocket(w http.ResponseWriter, r *http.Request) {

	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		return
	}

	err = srv.Poll.Add(conn)
	if err != nil {
		log.Printf("FAIL TO ADD CONNECTION")
		conn.Close()
	}
}

type PingMsg struct {
	Name  string
	Age   int
	Ready bool
}
