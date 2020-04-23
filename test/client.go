package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/binhgo/foosee/core"
	"github.com/gorilla/websocket"
)

var (
	ip          = flag.String("ip", "127.0.0.1", "server IP")
	connections = flag.Int("conn", 1, "number of websocket connections")
)

func main() {

	flag.Usage = func() {
		io.WriteString(os.Stderr, `Websockets client generator Example usage: ./client -ip=172.17.0.1 -conn=10`)
		flag.PrintDefaults()
	}
	flag.Parse()

	u := url.URL{Scheme: "ws", Host: *ip + ":8000", Path: "/"}
	log.Printf("Connecting to %s", u.String())

	var conns []*websocket.Conn
	for i := 0; i < *connections; i++ {
		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			fmt.Println("Failed to connect", i, err)
			break
		}
		conns = append(conns, c)
		defer func() {
			c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
			time.Sleep(time.Second)
			c.Close()
		}()
	}

	log.Printf("Finished initializing %d connections", len(conns))
	tts := time.Second
	if *connections > 100 {
		tts = time.Millisecond * 5
	}

	//
	go func() {
		for {
			for i := 0; i < len(conns); i++ {
				time.Sleep(tts)
				conn := conns[i]

				_, bb, err := conn.ReadMessage()
				if err != nil {
					fmt.Println(err.Error())
					continue
				}

				fmt.Println(string(bb))
			}
		}
	}()
	//

	for {
		for i := 0; i < len(conns); i++ {
			time.Sleep(tts)
			conn := conns[i]
			log.Printf("Conn %d sending message", i)
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second*5)); err != nil {
				fmt.Printf("Failed to receive pong: %v", err)
			}

			// conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Hello from conn %v", i)))

			data := Order{
				OrgCode:   "ghn",
				OrderCode: "O112233",
				ClientID:  "C1234",
				LoadedBy:  "Binh",
				UpdatedBy: "Vu",
				Version:   2,
			}

			req := core.Request{
				Action: "PUT-ORDER",
				Data:   data,
			}

			by, _ := json.Marshal(req)

			conn.WriteMessage(websocket.TextMessage, by)
		}
	}
}

type Order struct {
	OrgCode      string `json:"orgCode" bson:"org_code,omitempty"`
	OrderCode    string `json:"orderCode" bson:"order_code,omitempty"`
	ExternalCode string `json:"externalCode" bson:"external_code,omitempty"`
	OrderID      string `json:"orderId" bson:"order_id,omitempty"`
	ClientID     string `json:"clientId" bson:"client_id,omitempty"`
	LoadedBy     string `json:"loadedBy" bson:"loaded_by,omitempty"`
	UpdatedBy    string `json:"updatedBy" bson:"updated_by,omitempty"`
	Version      int    `json:"version,omitempty" bson:"version,omitempty"`
}
