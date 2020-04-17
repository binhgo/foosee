package example

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strconv"

	"github.com/binhgo/foosee/core"
	"github.com/gobwas/ws"
)

var srv *core.Server

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

func main() {

	srv = core.NewServer(-10)
	go srv.Start()

	srv.SetHandle("GET-ORDER", handleOrder)

	http.HandleFunc("/", UpgradeWebsocket)
	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func handleOrder(request core.Request) core.Response {

	by, err := json.Marshal(request.Data)
	if err != nil {
		return core.Response{
			Status:  "ERROR",
			Message: err.Error(),
		}
	}

	var data *PingMsg
	err = json.Unmarshal(by, &data)
	if err != nil {
		return core.Response{
			Status:  "ERROR",
			Message: err.Error(),
		}
	}

	md := data.Name
	md = md + "-" + strconv.Itoa(rand.Intn(100000))

	return core.Response{
		Status:  "OK",
		Message: md,
	}
}

type PingMsg struct {
	Name  string
	Age   int
	Ready bool
}
