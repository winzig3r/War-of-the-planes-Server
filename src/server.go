package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// We'll need to define an Upgrader
// this will require a Read and Write buffer size
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Player struct {
	transform string
	name      string
	websocket *websocket.Conn
}

type Room struct {
	players map[string]Player
}

var mutex = &sync.Mutex{}
var namesFileLocation = "C:/Users/vince/Programming/JavaScript/WebSocketServer/names.txt"
var port = 6942
var names []string
var allPlayerIds []string
var rooms = map[string]*Room{}
var playersWithoutRoom = map[string]*websocket.Conn{}

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Home Page")
}

func reader(conn *websocket.Conn) {
	for {
		// read in a message
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		// print out that message for clarity
		decodeClientMessage(p)
	}
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
	go reader(ws)
}

func setupRoutes() {
	http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", wsEndpoint)
}

func main() {
	names = getNamesFromFile(namesFileLocation)
	fmt.Println("Listening on localhost: " + strconv.Itoa(port))
	setupRoutes()
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}

func decodeClientMessage(message_raw []byte) {
	var message map[string]interface{}
	if json.Unmarshal(message_raw, &message) != nil {
		fmt.Println("Error decoding Message!")
	} else {
		mesageType := fmt.Sprintf("%v", message["type"])
		switch mesageType {
		case "createRoom":
			pId := fmt.Sprintf("%v", message["Id"])
			newRoomId := getRandomRoomId()
			playerName := fmt.Sprintf("%v", message["name"])
			if len(playerName) == 0 {
				rand.Seed(time.Now().UnixNano())
				playerName = names[rand.Intn(len(names)-1)]
			}
			newPlayer := Player{name: playerName, websocket: playersWithoutRoom[pId], transform: "0"}
			playerInfo := map[string]Player{pId: newPlayer}
			newRoom := Room{players: playerInfo}
			mutex.Lock()
			rooms[newRoomId] = &newRoom
			delete(playersWithoutRoom, pId)
			mutex.Unlock()
			broadcast(newRoomId, "{\"type\":\"nameList\", \"names\":"+string(getNamesInRoom(newRoomId))+"}")

			currentPlayer := rooms[newRoomId].players[pId]
			send(&currentPlayer, "{\"type\":\"createdRoom\", \"newRoomId\":\""+newRoomId+"\"}")
		case "joinRoom":
			pId := fmt.Sprintf("%v", message["Id"])
			roomId := fmt.Sprintf("%v", message["roomId"])
			playerName := fmt.Sprintf("%v", message["name"])
			if len(playerName) == 0 {
				rand.Seed(time.Now().UnixNano())
				playerName = names[rand.Intn(len(names)-1)]
			}
			if _, ok := rooms[roomId]; ok {
				//Setting up a new Player Object
				newPlayer := Player{name: playerName, websocket: playersWithoutRoom[pId], transform: "0"}
				//Moving the new Player Object into the room
				mutex.Lock()
				rooms[roomId].players[pId] = newPlayer
				//Deleting the playerId out of the playersWithoutRoom
				delete(playersWithoutRoom, pId)
				mutex.Unlock()
				//Informing the client itself and the clients who already were in the room
				broadcast(roomId, "{\"type\":\"nameList\", \"names\":"+string(getNamesInRoom(roomId))+"}")

				currentPlayer := rooms[roomId].players[pId]
				send(&currentPlayer, "{\"type\":\"joinSuccess\", \"newRoomId\":\""+roomId+"\"}")
			} else {
				currentPlayer := rooms[roomId].players[pId]
				send(&currentPlayer, "{\"type\":\"Error\", \"value\":\"NoSuchRoomId\"}")
			}
		case "transformUpdate":
			pId := fmt.Sprintf("%v", message["playerId"])
			roomId := fmt.Sprintf("%v", message["roomId"])
			//fmt.Println("Trying to update transforms in room " + roomId)
			modifiedPlayer := rooms[roomId].players[pId]
			modifiedPlayer.transform = fmt.Sprintf("%v", message["newTransform"])
			mutex.Lock()
			rooms[roomId].players[pId] = modifiedPlayer
			mutex.Unlock()
			updateClientTransforms(roomId)
		case "clientDisconnected":
			disconnectClient(fmt.Sprintf("%v", message["roomId"]), fmt.Sprintf("%v", message["Id"]))
		case "completeDelete":
			pId := fmt.Sprintf("%v", message["playerId"])
			roomId := fmt.Sprintf("%v", message["roomId"])
			if len(roomId) > 0 {
				disconnectClient(roomId, pId)
			}
			delete(playersWithoutRoom, pId)
		}
	}
}

func disconnectClient(roomId string, playerId string) {
	broadcast(roomId, "{\"type\":\"clientDisconnected\", \"Id\":\""+playerId+"\"}")
	mutex.Lock()
	playersWithoutRoom[playerId] = rooms[roomId].players[playerId].websocket
	delete(rooms[roomId].players, playerId)
	mutex.Unlock()
	fmt.Print("Player " + playerId + " disconnected")
	//Deleting the room if nobody is in it anymore
	if len(rooms[roomId].players) == 0 {
		mutex.Lock()
		delete(rooms, roomId)
		mutex.Unlock()
		fmt.Println(" and room " + roomId + " got deleted because nobody was in it anymore")
		return
	}
	fmt.Println()
}

func updateClientTransforms(roomId string) {
	transforms := map[string]string{}
	for k, v := range rooms[roomId].players {
		transforms[k] = v.transform
	}
	jsonString, e := json.Marshal(transforms)
	if e != nil {
		fmt.Println("Something went wrong with getting the transforms")
		return
	}
	broadcast(roomId, "{\"type\":\"updatePlayerTransform\",\"allPlayerTransformDict\":"+string(jsonString)+"}")
}

func getNamesInRoom(roomId string) []byte {
	var names = map[string]string{}
	for k, v := range rooms[roomId].players {
		names[k] = v.name
	}
	jsonString, e := json.Marshal(names)
	if e != nil {
		fmt.Println("Something went wrong with getting the names")
		return nil
	}
	return jsonString
}

func send(p *Player, message string) error {
	mutex.Lock()
	defer mutex.Unlock()
	return p.websocket.WriteMessage(1, []byte(message))
}

func broadcast(roomId string, message string) {
	for _, v := range rooms[roomId].players {
		v.websocket.WriteMessage(1, []byte(message))
		send(&v, message)
	}
}

func handleNewPlayer(conn *websocket.Conn) {
	newId := getNewPlayerId()
	conn.WriteMessage(1, []byte("{\"type\":\"setId\", \"newId\":\""+newId+"\"}"))
	fmt.Println("Client connected and has now Id: " + newId)
	playersWithoutRoom[newId] = conn
}

func getNewPlayerId() string {
	rand.Seed(time.Now().UnixNano())
	randomValue := "player " + strconv.Itoa(rand.Intn(999999-100000)+100000)
	if stringInSlice(randomValue, allPlayerIds) {
		return getNewPlayerId()
	} else {
		allPlayerIds = append(allPlayerIds, randomValue)
		return randomValue
	}
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func getRandomRoomId() string {
	rand.Seed(time.Now().UnixNano())
	alphabet := strings.ToUpper("abcdefghijklmnopqrstuvwxyz")
	roomId := ""
	for i := 0; i < 6; i++ {
		roomId = roomId + string(alphabet[rand.Intn(len(alphabet)-1)])
	}
	if _, ok := rooms[roomId]; ok {
		return getRandomRoomId()
	} else {
		return roomId
	}
}

func getNamesFromFile(file string) []string {
	var result = []string{}
	f, err := os.Open(file)

	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		result = append(result, string(scanner.Text()))
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	//return result[rand.Intn(len(result)-1)]
	return result
}
