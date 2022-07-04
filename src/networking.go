package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
)

// We'll need to define an Upgrader
// this will require a Read and Write buffer size
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Home Page")
}

func tcpReader(conn *websocket.Conn) {
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		decodeClientMessageOnTCP(p)
		//time.Sleep(10 * time.Millisecond)
	}
}

func updReader(pc net.PacketConn, addr net.Addr, buf []byte) {
	decodeClientMessageOnUDP(pc, addr, buf)
}

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	// upgrade this connection to a WebSocket
	// connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}
	handleNewPlayer(ws)
	if err != nil {
		log.Println(err)
	}
	// listen indefinitely for new messages coming
	// through on our WebSocket connection
	go tcpReader(ws)
}

func setupRoutes() {
	http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", wsEndpoint)
}

func startTCP() {
	fmt.Println("TCP Listening on Port: " + strconv.Itoa(PORT_TCP))
	setupRoutes()
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(PORT_TCP), nil))
}

func startUDP() {
	// listen to incoming udp packets
	pc, err := net.ListenPacket("udp", ":"+strconv.Itoa(PORT_UDP))
	fmt.Println("UDP Listening on Port: " + strconv.Itoa(PORT_UDP))
	if err != nil {
		log.Fatal(err)
	}
	defer pc.Close()
	for {
		buf := make([]byte, 1024)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			continue
		}
		go updReader(pc, addr, buf[:n])
		//time.Sleep(10 * time.Millisecond)
	}
}

func sendTCP(p *Player, message string) error {
	mutex.Lock()
	defer mutex.Unlock()
	if p.websocket == nil {
		return nil
	} else {
		return p.websocket.WriteMessage(1, []byte(message))
	}
}

func sendUDP(p *Player, message string) {
	mutex.Lock()
	defer mutex.Unlock()
	if p.udpConn != nil {
		p.udpConn.WriteTo([]byte(message), p.udpAddr)
	}
}

func broadcastTCP(roomId string, message string) {
	connectedPlayers := make(map[int]Player)
	mutex.Lock()
	if _, exists := rooms[roomId]; exists {
		for key, value := range rooms[roomId].players {
			if value.websocket != nil {
				connectedPlayers[key] = value
			}
		}
	} else {
		fmt.Println("No such room (", roomId, ") found in broadcastTCP()")
	}
	mutex.Unlock()
	for _, v := range connectedPlayers {
		if v.websocket != nil {
			sendTCP(&v, message)
		}
	}
}

func broadcastUDP(roomId string, message string) {
	connectedPlayers := make(map[int]Player)
	mutex.Lock()
	if _, exists := rooms[roomId]; exists {
		for key, value := range rooms[roomId].players {
			connectedPlayers[key] = value
		}
	} else {
		fmt.Println("No such room (" + roomId + ") found")
	}
	mutex.Unlock()
	for _, v := range connectedPlayers {
		sendUDP(&v, message)
	}
}
