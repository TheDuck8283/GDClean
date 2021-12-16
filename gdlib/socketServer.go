package gdlib

import (
	"encoding/json"
	"fmt"
	"gdl-s/gdlib/events"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type    int            `json:"type"`
	Id      string         `json:"id"`
	Payload PayloadRequest `json:"payload"`
}

type PayloadRequest struct {
	Data []byte `json:"data"`
}

type SocketResponse struct {
	Data interface{} `json:"data"`
	Id   string      `json:"id"`
}

var upgrader = websocket.Upgrader{}
var semaphore = make(chan int, 1)

func handleRequest(w http.ResponseWriter, r *http.Request) {
	semaphore <- 1
	fmt.Println("Joined", r.RemoteAddr)
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	c, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Fatal("Upgrade:", err)
	}

	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		processEvent(message, mt, c)
	}
	<-semaphore
}

func sendErrorResponse(err error) []byte {
	errorResp, err := json.Marshal(struct {
		Err string `json:"error"`
	}{
		Err: err.Error(),
	})
	if err != nil {
		log.Fatal("error:", err)
	}
	return errorResp
}

func StartServer() {
	http.HandleFunc("/v1", handleRequest)
	fmt.Println("GDLib listening on port 7890")
	log.Fatal(http.ListenAndServe("localhost:7890", nil))
}

func processEvent(payload []byte, mt int, c *websocket.Conn) {
	var err error
	var message Message
	err = json.Unmarshal(payload, &message)
	if err != nil {
		c.WriteMessage(mt, sendErrorResponse(err))
		return
	}

	fmt.Println("Received:", message.Id)
	var response interface{}
	switch message.Type {
	case events.Ping:
		response, err = processPing(message.Payload.Data)
	case events.MurmurHash2:
		response, err = processMurmurHash2(message.Payload.Data)
	case events.Quit:
		response, err = processQuit(message.Payload.Data)
	case events.FSWatcher:
		response, err = processFSWatcher(message.Payload.Data, c)
	}

	if err != nil {
		c.WriteMessage(mt, sendErrorResponse(err))
		return
	}

	newResp := SocketResponse{
		Data: response,
		Id:   message.Id,
	}

	marshaled, err := json.Marshal(newResp)
	if err != nil {
		c.WriteMessage(mt, sendErrorResponse(err))
		return
	}
	fmt.Printf("Response: %+v\n", newResp)
	err = c.WriteMessage(mt, []byte(marshaled))
	if err != nil {
		c.WriteMessage(mt, sendErrorResponse(err))
		return
	}
}

func processPing(payload []byte) ([]byte, error) {
	fmt.Printf("%v", payload)
	return nil, nil
}

func processMurmurHash2(payload []byte) (uint32, error) {
	hashmap := make(map[string]string)
	err := json.Unmarshal(payload, &hashmap)
	if err != nil {
		return 0, err
	}

	filePath := hashmap["filePath"]
	fd, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	stat, err := fd.Stat()
	if err != nil {
		return 0, err
	}
	defer fd.Close()
	buffer := make([]byte, stat.Size())
	_, err = fd.Read(buffer)
	if err != nil {
		return 0, err
	}

	res := buffer[:0]
	for _, v := range buffer {
		if v != 9 && v != 10 && v != 13 && v != 32 {
			res = append(res, v)
		}
	}

	murmur2 := MurmurHash2(res)

	return murmur2, nil
}

func processQuit(payload []byte) (uint32, error) {
	os.Exit(0)
	return 0, nil
}

func processFSWatcher(payload []byte, c *websocket.Conn) (uint32, error) {
	go startListener()
	return 0, nil
}
