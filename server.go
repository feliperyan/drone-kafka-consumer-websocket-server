package main

// 1. Consume kafka messages
// 2. Parse format into json?
// 3. Broadcast to websocket clients
// 4. Allow client to determine what message group it wants?

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

type gpsCoord struct {
	Lat float64
	Lon float64
}

type droneMessage struct {
	CurrentPosition gpsCoord   `json:"CurrentPosition"`
	Destinations    []gpsCoord `json:"Destinations"`
	NextDestination int        `json:"NextDestination"`
	Speed           float64    `json:"Speed"`
	Name            string     `json:"Name"`
}

var (
	upgrader    = websocket.Upgrader{}
	allMessages chan droneMessage
	closeConn   chan *websocket.Conn
	allConns    chan *websocket.Conn
	addr        = flag.String("port", "8080", "http service address")
)

func init() {
	allMessages = make(chan droneMessage)
	closeConn = make(chan *websocket.Conn)
	allConns = make(chan *websocket.Conn)
}

func greet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World! %s", time.Now())
}

func incomingWebsocket(wri http.ResponseWriter, req *http.Request) {
	newConn, err := upgrader.Upgrade(wri, req, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer newConn.Close()

	allConns <- newConn

	for {
		mt, msg, err := newConn.ReadMessage()
		if err != nil {
			log.Println("Error: ", err)
			closeConn <- newConn
			return
		}
		if mt == websocket.TextMessage {
			fmt.Println(string(msg))
		}
	}

}

func processMessages(srv *http.Server, socks chan *websocket.Conn, messages chan droneMessage, leaving chan *websocket.Conn) {
	allSocks := make([]*websocket.Conn, 0)

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)

	for {
		select {
		case sig := <-sigint:
			fmt.Println("\nKilling server: ", sig)
			for _, c := range allSocks {
				err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					log.Println("write: ", err)
				}
			}
			if err := srv.Shutdown(context.Background()); err != nil {
				log.Printf("HTTP server Shutdown Error: %v", err)
			}

		case c := <-leaving:
			for i, v := range allSocks {
				if v == c { // remove
					allSocks[i] = allSocks[len(allSocks)-1]
					allSocks[len(allSocks)-1] = nil
					allSocks = allSocks[:len(allSocks)-1]
				}
			}
			fmt.Println("Client left. Total Clients: ", len(allSocks))

		case msg := <-messages:
			fmt.Println("received: ", msg)
			for _, v := range allSocks {
				v.WriteJSON(msg)
			}

		case conn := <-socks:
			allSocks = append(allSocks, conn)
			fmt.Println("Client joined. Total Clients: ", len(allSocks))
		}
	}
}

func kafkaReceiver(messages chan droneMessage) {
	fmt.Println("KafkaReceiver goroutine")
	theReader := initialiseKafkaReader(usingTLS)
	defer theReader.Close()

	for {
		m, err := theReader.ReadMessage(context.Background())
		if err != nil {
			break
		}
		fmt.Printf("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))
		droMsg := droneMessage{}
		err = json.Unmarshal(m.Value, &droMsg)
		if err != nil {
			fmt.Println("JSON unmarshalling error: ", err)
		} else {
			messages <- droMsg
		}

	}

}

func main() {

	flag.Parse()
	log.SetFlags(0)

	theAddr := fmt.Sprintf(":%s", *addr)
	fmt.Println("Starting: ", theAddr)

	serv := &http.Server{Addr: theAddr, Handler: nil}
	go processMessages(serv, allConns, allMessages, closeConn)
	go kafkaReceiver(allMessages)

	http.HandleFunc("/greet", greet)
	http.HandleFunc("/", incomingWebsocket)
	log.Fatal(serv.ListenAndServe())

}
