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
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"

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
	dev         = flag.Bool("dev", false, "whether to accept websockets from any origin")
)

func init() {
	allMessages = make(chan droneMessage)
	closeConn = make(chan *websocket.Conn)
	allConns = make(chan *websocket.Conn)
}

func indexWS(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadFile("websocket.html")
	if err != nil {
		fmt.Println("Could not open file.", err)
	}
	fmt.Fprintf(w, "%s", content)
}

func incomingWebsocket(wri http.ResponseWriter, req *http.Request) {
	log.Print("incoming ws req origin: ", req.Header["Origin"])
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
			fmt.Println("From Client: ", string(msg))
		}
	}

}

func processMessages(srv *http.Server, socks chan *websocket.Conn, messages chan droneMessage, leaving chan *websocket.Conn) {
	allSocks := make([]*websocket.Conn, 0)

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)

	for {
		select {
		case sig := <-sigint: // server going down
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
			return

		case c := <-leaving: //client closed conn
			for i, v := range allSocks {
				if v == c { // remove client for slice
					allSocks[i] = allSocks[len(allSocks)-1]
					allSocks[len(allSocks)-1] = nil
					allSocks = allSocks[:len(allSocks)-1]
				}
			}
			fmt.Println("Client left. Total Clients: ", len(allSocks))

		case msg := <-messages: // broadcast kafka message to clients
			fmt.Println("received: ", msg)
			for _, v := range allSocks {
				v.WriteJSON(msg)
			}

		case conn := <-socks: // new client connected, add to slice
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

	if *dev == true {
		fmt.Println("Running in dev. Accepting websockets from any origin.")
		upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	}

	theAddr := fmt.Sprintf(":%s", *addr)
	fmt.Println("Starting: ", theAddr)

	serv := &http.Server{Addr: theAddr, Handler: nil}
	go processMessages(serv, allConns, allMessages, closeConn)
	go kafkaReceiver(allMessages)

	http.HandleFunc("/", indexWS)
	http.HandleFunc("/ws", incomingWebsocket)
	log.Fatal(serv.ListenAndServe())

}
