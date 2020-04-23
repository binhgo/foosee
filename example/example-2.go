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

var app *core.App

func UpgradeWebsocket_2(w http.ResponseWriter, r *http.Request) {
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

	app = core.NewApp("order-service")
	srv, err := app.SetupAPIServer()
	if err != nil {
		log.Fatal(err)
	}

	srv.SetHandle("GET-ORDER", handleOrder_2)

	http.HandleFunc("/", UpgradeWebsocket_2)
	err = http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal(err)
	}

	err = app.Launch()
	if err != nil {
		log.Fatal(err)
	}
}

func handleOrder_2(request core.Request) core.Response {

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
