package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
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
	transform     string
	name          string
	currentHealth int
	websocket     *websocket.Conn
	udpConn       net.PacketConn
	udpAddr       net.Addr
}

type Room struct {
	players map[int]Player
}

const PORT_UDP = 6943
const PORT_TCP = 6942

var mutex = &sync.Mutex{}

var namesFileLocation = "names.txt"
var names []string
var allPlayerIds []int
var rooms = map[string]*Room{}
var playersWithoutRoom = map[int]*websocket.Conn{}

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

func main() {
	//fmt.Println("Penis123")
	go startTCP()
	startUDP()
}

func startTCP() {
	names = getNamesFromFile(namesFileLocation)
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
	}
}

func decodeClientMessageOnUDP(udpConnection net.PacketConn, addr net.Addr, message_raw []byte) {
	var message map[string]interface{}
	if json.Unmarshal(message_raw, &message) != nil {
		fmt.Println("Error decoding Message!")
	} else {
		mesageType := fmt.Sprintf("%v", message["type"])
		switch mesageType {
		case "transformUpdate":
			pId, _ := strconv.Atoi(fmt.Sprintf("%v", message["playerId"]))
			roomId := fmt.Sprintf("%v", message["roomId"])
			//fmt.Println("Trying to update transform of player " + strconv.Itoa(pId) + " the new Transform is: " + fmt.Sprintf("%v", message["newTransform"]))
			mutex.Lock()
			//Setting the connection data if it is a new Connection
			if rooms[roomId].players[pId].udpConn == nil {
				movingPlayer := rooms[roomId].players[pId]
				movingPlayer.udpConn = udpConnection
				movingPlayer.udpAddr = addr
				rooms[roomId].players[pId] = movingPlayer
			}
			//Udpating the transform
			modifiedPlayer := rooms[roomId].players[pId]
			modifiedPlayer.transform = fmt.Sprintf("%v", message["newTransform"])
			rooms[roomId].players[pId] = modifiedPlayer
			mutex.Unlock()
			//Informing the other clients
			updateClientTransforms(roomId)
		}
	}
}

func decodeClientMessageOnTCP(message_raw []byte) {
	var message map[string]interface{}
	if json.Unmarshal(message_raw, &message) != nil {
		fmt.Println("Error decoding Message: " + string(message_raw))
	} else {
		mesageType := fmt.Sprintf("%v", message["type"])
		switch mesageType {
		case "createRoom":
			pId, _ := strconv.Atoi(fmt.Sprintf("%v", message["Id"]))
			newRoomId := getRandomRoomId()
			playerName := fmt.Sprintf("%v", message["name"])
			if len(playerName) == 0 {
				rand.Seed(time.Now().UnixNano())
				playerName = names[rand.Intn(len(names)-1)]
			}
			newPlayer := Player{name: playerName, websocket: playersWithoutRoom[pId], transform: "0", currentHealth: 200}
			playerInfo := map[int]Player{pId: newPlayer}
			newRoom := Room{players: playerInfo}
			mutex.Lock()
			rooms[newRoomId] = &newRoom
			delete(playersWithoutRoom, pId)
			mutex.Unlock()
			broadcastTCP(newRoomId, "{\"type\":\"otherPlayerData\", \"names\":"+string(getNamesInRoom(newRoomId))+", \"healthValues\":"+string(getHealthInRoom(newRoomId))+"}")

			currentPlayer := rooms[newRoomId].players[pId]
			sendTCP(&currentPlayer, "{\"type\":\"createdRoom\", \"newRoomId\":\""+newRoomId+"\", \"startHealth\":\""+strconv.Itoa(newPlayer.currentHealth)+"\"}")
		case "joinRoom":
			pId, _ := strconv.Atoi(fmt.Sprintf("%v", message["Id"]))
			roomId := fmt.Sprintf("%v", message["roomId"])
			playerName := fmt.Sprintf("%v", message["name"])

			if len(playerName) == 0 {
				rand.Seed(time.Now().UnixNano())
				playerName = names[rand.Intn(len(names)-1)]
			}
			if _, ok := rooms[roomId]; ok {
				//Setting up a new Player Object
				newPlayer := Player{name: playerName, websocket: playersWithoutRoom[pId], transform: "0", currentHealth: 100}
				//Moving the new Player Object into the room
				mutex.Lock()
				rooms[roomId].players[pId] = newPlayer
				//Deleting the playerId out of the playersWithoutRoom
				delete(playersWithoutRoom, pId)
				mutex.Unlock()
				//Informing the client itself and the clients who already were in the room
				broadcastTCP(roomId, "{\"type\":\"otherPlayerData\", \"names\":"+string(getNamesInRoom(roomId))+", \"healthValues\":"+string(getHealthInRoom(roomId))+"}")

				currentPlayer := rooms[roomId].players[pId]
				sendTCP(&currentPlayer, "{\"type\":\"joinSuccess\", \"newRoomId\":\""+roomId+"\", \"startHealth\":\""+strconv.Itoa(newPlayer.currentHealth)+"\"}")
			} else {
				currentPlayer := rooms[roomId].players[pId]
				sendTCP(&currentPlayer, "{\"type\":\"Error\", \"value\":\"NoSuchRoomId\"}")
			}
		case "shootBulletRequest":
			//Getting generall information about the bullet
			roomId := fmt.Sprintf("%v", message["roomId"])
			bulletType := fmt.Sprintf("%v", message["bulletType"])
			shooter := fmt.Sprintf("%v", message["shooter"])
			//Getting the startPositioin
			startPos := (message["bulletStartPosition"]).([]interface{})
			startPosVal, _ := json.Marshal(startPos)

			//Getting the starting velocity of the bullet
			velocity := (message["velocity"]).([]interface{})
			velocityVal, _ := json.Marshal(velocity)

			//Getting direction the bullet has to fly towards
			planeFacingDirection := (message["planeFacingDirection"]).([]interface{})
			planeFacingDirectionVal, _ := json.Marshal(planeFacingDirection)

			//Updating the clients in the room
			broadcastTCP(roomId, "{\"type\":\"bulletShot\", \"bulletType\":\""+bulletType+"\", \"shooter\":\""+shooter+"\", \"startPos\":"+string(startPosVal)+", \"velocity\":"+string(velocityVal)+", \"planeFacingDirection\":"+string(planeFacingDirectionVal)+"}")
		case "shootRocketRequest":
			//Getting the Id of the room the bullet was shoot in
			roomId := fmt.Sprintf("%v", message["roomId"])
			rocketType := fmt.Sprintf("%v", message["rocketType"])
			shooter := fmt.Sprintf("%v", message["shooter"])
			target := fmt.Sprintf("%v", message["target"])

			//Getting the start Position to know where the rocket needs to be spawned
			startPos := (message["bulletStartPosition"]).([]interface{})
			startPosVal, _ := json.Marshal(startPos)

			//Getting the starting velocity of the rocket
			velocity := (message["velocity"]).([]interface{})
			velocityVal, _ := json.Marshal(velocity)

			//Getting direction the rocket has to fly towards
			planeFacingDirection := (message["planeFacingDirection"]).([]interface{})
			planeFacingDirectionVal, _ := json.Marshal(planeFacingDirection)

			//fmt.Println("Server End  : " + "[\"" + xEnd + "\", \"" + yEnd + "\", \"" + zEnd + "\"]")
			//Updating the clients in the room
			broadcastTCP(roomId, "{\"type\":\"rocketShot\", \"rocketType\":\""+rocketType+"\", \"shooter\":\""+shooter+"\", \"startPos\":"+string(startPosVal)+", \"velocity\":"+string(velocityVal)+", \"facingAngle\":"+string(planeFacingDirectionVal)+", \"targetId\":\""+target+"\"}")
		case "playerHit":
			roomId := fmt.Sprintf("%v", message["roomId"])
			playerId, _ := strconv.Atoi(fmt.Sprintf("%v", message["playerId"]))
			//shooterId := fmt.Sprintf("%v", message["shooterId"])
			damage, _ := strconv.Atoi(fmt.Sprintf("%v", message["damage"]))

			mutex.Lock()
			shotPlayer := rooms[roomId].players[playerId]
			shotPlayer.currentHealth -= damage
			rooms[roomId].players[playerId] = shotPlayer
			mutex.Unlock()

			broadcastTCP(roomId, "{\"type\":\"playerHit\", \"hitPlayerId\":\""+strconv.Itoa(playerId)+"\", \"newHealth\":\""+strconv.Itoa(shotPlayer.currentHealth)+"\",}")
		case "clientDisconnected":
			disconnectedPlayerId, _ := strconv.Atoi(fmt.Sprintf("%v", message["Id"]))
			disconnectClient(fmt.Sprintf("%v", message["roomId"]), disconnectedPlayerId)
		case "completeDelete":
			pId, _ := strconv.Atoi(fmt.Sprintf("%v", message["playerId"]))
			roomId := fmt.Sprintf("%v", message["roomId"])
			if len(roomId) > 0 {
				disconnectClient(roomId, pId)
			}
			delete(playersWithoutRoom, pId)
		}
	}
}

func disconnectClient(roomId string, playerId int) {
	broadcastTCP(roomId, "{\"type\":\"clientDisconnected\", \"Id\":\""+strconv.Itoa(playerId)+"\"}")
	mutex.Lock()
	playersWithoutRoom[playerId] = rooms[roomId].players[playerId].websocket
	delete(rooms[roomId].players, playerId)
	mutex.Unlock()
	fmt.Print("Player " + strconv.Itoa(playerId) + " disconnected")
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
	mutex.Lock()
	transforms := map[int]string{}
	playersCopy := &rooms[roomId].players
	for k, v := range *playersCopy {
		if len(v.transform) > 1 {
			transforms[k] = v.transform
		}
	}
	mutex.Unlock()
	jsonString, e := json.Marshal(transforms)
	if e != nil {
		fmt.Println("Something went wrong with getting the transforms")
		return
	}
	broadcastUDP(roomId, "{\"type\":\"updatePlayerTransform\",\"allPlayerTransformDict\":"+string(jsonString)+"}")
}

func getNamesInRoom(roomId string) []byte {
	var names = map[int]string{}
	mutex.Lock()
	for playerId, player := range rooms[roomId].players {
		names[playerId] = player.name
	}
	mutex.Unlock()
	jsonString, e := json.Marshal(names)
	if e != nil {
		fmt.Println("Something went wrong with getting the names")
		return nil
	}
	return jsonString
}

func getHealthInRoom(roomId string) []byte {
	var allPlayerHealth = map[int]string{}
	mutex.Lock()
	for playerId, player := range rooms[roomId].players {
		allPlayerHealth[playerId] = strconv.Itoa(player.currentHealth)
	}
	mutex.Unlock()
	jsonString, e := json.Marshal(allPlayerHealth)
	if e != nil {
		fmt.Println("Something went wrong with getting the health")
		return nil
	}
	return jsonString
}

func sendTCP(p *Player, message string) error {
	mutex.Lock()
	defer mutex.Unlock()
	return p.websocket.WriteMessage(1, []byte(message))
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
	for key, value := range rooms[roomId].players {
		connectedPlayers[key] = value
	}
	mutex.Unlock()
	for _, v := range connectedPlayers {
		sendTCP(&v, message)
	}
}

func broadcastUDP(roomId string, message string) {
	connectedPlayers := make(map[int]Player)
	mutex.Lock()
	for key, value := range rooms[roomId].players {
		connectedPlayers[key] = value
	}
	mutex.Unlock()
	for _, v := range connectedPlayers {
		sendUDP(&v, message)
	}
}

func handleNewPlayer(conn *websocket.Conn) {
	newId := getNewPlayerId()
	conn.WriteMessage(1, []byte("{\"type\":\"setId\", \"newId\":\""+strconv.Itoa(newId)+"\"}"))
	fmt.Println("Client connected and has now Id: " + strconv.Itoa(newId))
	playersWithoutRoom[newId] = conn
}

func getNewPlayerId() int {
	var newId int
	if len(allPlayerIds) > 0 {
		newId = allPlayerIds[len(allPlayerIds)-1] + 1
	} else {
		newId = 69
	}
	allPlayerIds = append(allPlayerIds, newId)
	return allPlayerIds[len(allPlayerIds)-1]
}

func getRandomRoomId() string {
	return "AAAAAA"
	/*
		ENABLE AFTER ENTKÄFERUNG
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
	*/
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
