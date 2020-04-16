package util

import (
	"net"
	"reflect"

	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func GetFD1(conn *websocket.Conn) uint64 {
	if conn != nil {
		connVal := reflect.Indirect(reflect.ValueOf(conn)).FieldByName("conn").Elem()
		tcpConn := reflect.Indirect(connVal).FieldByName("conn")
		fdVal := tcpConn.FieldByName("fd")
		pfdVal := reflect.Indirect(fdVal).FieldByName("pfd")
		i64 := pfdVal.FieldByName("Sysfd").Int()

		return uint64(i64)
	}

	return 0
}

func GetFD2(conn net.Conn) uint64 {
	if conn != nil {
		tcpConn := reflect.Indirect(reflect.ValueOf(conn)).FieldByName("conn")
		fdVal := tcpConn.FieldByName("fd")
		pfdVal := reflect.Indirect(fdVal).FieldByName("pfd")

		return uint64(pfdVal.FieldByName("Sysfd").Int())
	}

	return 0
}

func FromJson(data []byte, v interface{}) error {
	err := json.Unmarshal(data, v)
	return err
}

func ToJson(object interface{}) ([]byte, error) {
	b, err := json.Marshal(&object)
	return b, err
}
