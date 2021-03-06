package socket

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"reflect"
	"time"
)

type Message struct {
	Rules   map[string]string `json:"meta"`
	Content interface{}       `json:"content"`
}

type response struct {
	StatusCode string      `json:"statusCode"`
	Result     interface{} `result`
}

func business(conn Conn, data []byte) {
	// flag := false
	var message Message
	err := json.Unmarshal(data, &message)
	if err != nil {
		Log("json.Unmarshal()", err)
	}
	for _, v := range Routers {
		pred := v[0]
		act := v[1]
		if pred.(func(entry Message) bool)(message) {
			result := act.(Controller).Excute(message)
			_, err := writeResult(conn, result)
			if err != nil {
				Log("conn.WriteResult()", err)
			}
			return
		}
	}

	_, err = writeError(conn, "1111", "不能处理此类型的业务")
	if err != nil {
		Log("conn.WriteError()", err)
	}
}

func reader(conn Conn, readerChannel <-chan []byte, timeout int) {
	for {
		select {
		case data := <-readerChannel:
			conn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
			business(conn, data)
			break
		case <-time.After(time.Duration(timeout) * time.Second):
			conn.Close()
			Log("connection is closed.")
			return
		}
	}
}

// HandleConnection 处理长连接
func HandleConnection(conn Conn, timeout int) {
	//声明一个临时缓冲区，用来存储被截断的数据
	var tmpBuffer []byte

	//声明一个管道用于接收解包的数据
	readerChannel := make(chan []byte, 16)
	go reader(conn, readerChannel, timeout)

	buffer := make([]byte, 1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				continue
			}
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				Log("exit goroutine.")
				return
			}
			Log(conn.RemoteAddr().String(), " connection error: ", err, reflect.TypeOf(err))
			return
		}
		tmpBuffer = unpack(append(tmpBuffer, buffer[:n]...), readerChannel)
	}

}

func Log(v ...interface{}) {
	log.Println(v...)
}

// 路由
var Routers [][2]interface{}

// Route 路由注册
func Route(rule interface{}, controller Controller) {
	switch rule.(type) {
	case func(entry Message) bool:
		{
			var arr [2]interface{}
			arr[0] = rule
			arr[1] = controller
			Routers = append(Routers, arr)
		}
	case map[string]string:
		{
			defaultJudge := func(entry Message) bool {
				for ruleKey, ruleValue := range rule.(map[string]string) {
					val, ok := entry.Rules[ruleKey]
					if !ok {
						return false
					}
					if val != ruleValue {
						return false
					}
				}
				return true
			}
			var arr [2]interface{}
			arr[0] = defaultJudge
			arr[1] = controller
			Routers = append(Routers, arr)
		}
	default:
		Log("Something is wrong in Router")
	}
}

func init() {
	Routers = make([][2]interface{}, 0, 10)
}
