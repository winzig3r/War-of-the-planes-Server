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
	transform     string
	name          string
	planeType     string
	currentTeam   string
	playerId      string
	currentHealth int
	kills         int
	isNew         bool
	isDead        bool
	isReady       bool
	websocket     *websocket.Conn
	udpConn       net.PacketConn
	udpAddr       net.Addr
}

type RoomBase struct {
	sceneIndex     string
	roomRules      map[string]string
	availableTeams []string
	players        map[int]Player
}

const PORT_UDP = 9535
const PORT_TCP = 9536

var mutex = &sync.Mutex{}

var namesFileLocation = "names.txt"
var names []string
var allPlayerIds []int
var rooms = map[string]*RoomBase{}
var playersWithoutRoom = map[int]Player{}

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

func main() {
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
		//time.Sleep(10 * time.Millisecond)
	}
}

func decodeClientMessageOnUDP(udpConnection net.PacketConn, addr net.Addr, message_raw []byte) {
	var message map[string]interface{}
	if json.Unmarshal(message_raw, &message) != nil {
		fmt.Println("Error decoding Message: " + string(message_raw))
	} else {
		mesageType := fmt.Sprintf("%v", message["type"])
		switch mesageType {
		case "transformUpdate":
			playerId, _ := strconv.Atoi(fmt.Sprintf("%v", message["playerId"]))
			roomId := fmt.Sprintf("%v", message["roomId"])
			//fmt.Println("Trying to update transform of player " + strconv.Itoa(pId) + " the new Transform is: " + fmt.Sprintf("%v", message["newTransform"]))
			mutex.Lock()
			//Setting the connection data if it is a new Connection

			if _, roomExists := rooms[roomId]; roomExists {
				if _, playerExists := rooms[roomId].players[playerId]; playerExists {
					if rooms[roomId].players[playerId].udpConn == nil {
						movingPlayer := rooms[roomId].players[playerId]
						movingPlayer.udpConn = udpConnection
						movingPlayer.udpAddr = addr
						rooms[roomId].players[playerId] = movingPlayer
					}
					//Udpating the transform
					modifiedPlayer := rooms[roomId].players[playerId]
					modifiedPlayer.transform = fmt.Sprintf("%v", message["newTransform"])
					rooms[roomId].players[playerId] = modifiedPlayer
					mutex.Unlock()
					//Informing the other clients
					updateClientTransforms(roomId)
				} else {
					mutex.Unlock()
				}
			} else {
				mutex.Unlock()
			}
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
			playerId, _ := strconv.Atoi(fmt.Sprintf("%v", message["Id"]))
			newRoomId := getRandomRoomId()
			playerName := fmt.Sprintf("%v", message["name"])
			planeType := fmt.Sprintf("%v", message["planeType"])
			startHealth, _ := strconv.Atoi(fmt.Sprintf("%v", message["startHealth"]))
			selectedWorld := fmt.Sprintf("%v", message["worldIndex"])
			gameModeInfo := convertMap(message["gameModeInfo"].(map[string]interface{}))
			teams := strings.Split(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(gameModeInfo["teamColors"], "[", ""), "]", ""), "\"", ""), " ")
			if len(newRoomId) == 0 {
				affectedPlayer := playersWithoutRoom[playerId]
				sendTCP(&affectedPlayer, "{\"type\":\"Error\", \"value\":\"A room with that room Id was already created\",}")
				return
			}
			if len(playerName) == 0 {
				rand.Seed(time.Now().UnixNano())
				playerName = names[rand.Intn(len(names)-1)]
			}
			newPlayer := Player{}
			if playersWithoutRoom[playerId].isNew {
				newPlayer = Player{playerId: playersWithoutRoom[playerId].playerId, name: playerName, currentTeam: teams[rand.Intn(len(teams))], websocket: playersWithoutRoom[playerId].websocket, transform: "0", currentHealth: startHealth, planeType: planeType, isNew: false, kills: 0}
			} else {
				newPlayer = playersWithoutRoom[playerId]
				newPlayer.currentHealth = startHealth
				newPlayer.planeType = planeType
				newPlayer.name = playerName
				newPlayer.currentTeam = rooms[newRoomId].availableTeams[rand.Intn(len(rooms[newRoomId].availableTeams))]
			}
			playerInfo := map[int]Player{playerId: newPlayer}
			newRoom := RoomBase{players: playerInfo, sceneIndex: selectedWorld, availableTeams: teams, roomRules: gameModeInfo}
			mutex.Lock()
			rooms[newRoomId] = &newRoom
			delete(playersWithoutRoom, playerId)
			mutex.Unlock()
			currentPlayer := rooms[newRoomId].players[playerId]
			sendTCP(&currentPlayer, "{\"type\":\"clientConnected\", \"Id\":\""+strconv.Itoa(playerId)+"\", \"Name\":\""+playerName+"\", \"Team\":\""+newRoom.players[playerId].currentTeam+"\", \"IsReady\":\""+strconv.FormatBool(newRoom.players[playerId].isReady)+"\", \"PlaneType\":\""+planeType+"\", \"PlayerHealth\":\""+strconv.Itoa(newPlayer.currentHealth)+"\"}")
			sendTCP(&currentPlayer, "{\"type\":\"createdRoom\", \"newRoomId\":\""+newRoomId+"\", \"startHealth\":\""+strconv.Itoa(newPlayer.currentHealth)+"\", \"sceneIndex\":\""+selectedWorld+"\"}")
		case "joinRoom":
			playerId, _ := strconv.Atoi(fmt.Sprintf("%v", message["Id"]))
			roomId := fmt.Sprintf("%v", message["roomId"])
			playerName := fmt.Sprintf("%v", message["name"])
			planeType := fmt.Sprintf("%v", message["planeType"])
			startHealth, _ := strconv.Atoi(fmt.Sprintf("%v", message["startHealth"]))
			//Checking if the room is already full
			if hasPlayerLimit, _ := strconv.ParseBool(rooms[roomId].roomRules["hasMaxPlayers"]); hasPlayerLimit {
				if maxPlayerAmount, _ := strconv.Atoi(rooms[roomId].roomRules["maxPlayerCount"]); len(rooms[roomId].players) >= maxPlayerAmount {
					affectedPlayer := playersWithoutRoom[playerId]
					sendTCP(&affectedPlayer, "{\"type\":\"Error\", \"value\":\"The room is full\"}")
					return
				}
			}
			if len(playerName) == 0 {
				rand.Seed(time.Now().UnixNano())
				playerName = names[rand.Intn(len(names)-1)]
			}

			if _, ok := rooms[roomId]; ok {
				//Setting up a new Player Object
				newPlayer := Player{}
				if playersWithoutRoom[playerId].isNew {
					newPlayer = Player{playerId: playersWithoutRoom[playerId].playerId, name: playerName, currentTeam: rooms[roomId].availableTeams[rand.Intn(len(rooms[roomId].availableTeams))], websocket: playersWithoutRoom[playerId].websocket, transform: "0", currentHealth: startHealth, planeType: planeType, isNew: false, kills: 0}
				} else {
					newPlayer = playersWithoutRoom[playerId]
					newPlayer.currentHealth = startHealth
					newPlayer.planeType = planeType
					newPlayer.name = playerName
					newPlayer.currentTeam = rooms[roomId].availableTeams[rand.Intn(len(rooms[roomId].availableTeams))]
				}
				//Moving the new Player Object into the room
				mutex.Lock()
				rooms[roomId].players[playerId] = newPlayer
				//Deleting the playerId out of the playersWithoutRoom
				delete(playersWithoutRoom, playerId)
				mutex.Unlock()
				//Informing the client itself and the clients who already were in the room of the join event
				currentPlayer := rooms[roomId].players[playerId]
				broadcastTCP(roomId, "{\"type\":\"clientConnected\", \"Id\":\""+strconv.Itoa(playerId)+"\", \"Name\":\""+playerName+"\", \"Team\":\""+rooms[roomId].players[playerId].currentTeam+"\", \"IsReady\":\""+strconv.FormatBool(rooms[roomId].players[playerId].isReady)+"\", \"PlaneType\":\""+planeType+"\", \"PlayerHealth\":\""+strconv.Itoa(newPlayer.currentHealth)+"\"}")
				sendTCP(&currentPlayer, "{\"type\":\"joinSuccess\", \"newRoomId\":\""+roomId+"\", \"startHealth\":\""+strconv.Itoa(newPlayer.currentHealth)+"\", \"sceneIndex\":\""+rooms[roomId].sceneIndex+"\", \"gameMode\":\""+rooms[roomId].roomRules["gameModeType"]+"\", \"otherClients\":"+getOtherClientData(roomId)+"}")
			} else {
				currentPlayer := playersWithoutRoom[playerId]
				sendTCP(&currentPlayer, "{\"type\":\"Error\", \"value\":\"NoSuchRoomId\"}")
			}
		case "ready":
			roomId := fmt.Sprintf("%v", message["roomId"])
			playerId, _ := strconv.Atoi(fmt.Sprintf("%v", message["Id"]))
			mutex.Lock()
			affectedPlayer := rooms[roomId].players[playerId]
			affectedPlayer.isReady = true
			rooms[roomId].players[playerId] = affectedPlayer
			mutex.Unlock()
			broadcastTCP(roomId, string(message_raw))
		case "unready":
			roomId := fmt.Sprintf("%v", message["roomId"])
			playerId, _ := strconv.Atoi(fmt.Sprintf("%v", message["Id"]))
			mutex.Lock()
			affectedPlayer := rooms[roomId].players[playerId]
			affectedPlayer.isReady = false
			rooms[roomId].players[playerId] = affectedPlayer
			mutex.Unlock()
			broadcastTCP(roomId, string(message_raw))
		case "startGame":
			roomId := fmt.Sprintf("%v", message["roomId"])
			fmt.Println("Room", roomId, "wants to start the game")
			broadcastTCP(roomId, string(message_raw))
		case "rejoin":
			playerId, _ := strconv.Atoi(fmt.Sprintf("%v", message["playerId"]))
			roomId := fmt.Sprintf("%v", message["roomId"])
			newHealth, _ := strconv.Atoi(fmt.Sprintf("%v", message["newHealth"]))
			mutex.Lock()
			rejoiningPlayer := rooms[roomId].players[playerId]
			rejoiningPlayer.currentHealth = newHealth
			rejoiningPlayer.isDead = false
			rooms[roomId].players[playerId] = rejoiningPlayer
			mutex.Unlock()
			broadcastTCP(roomId, "{\"type\":\"rejoin\", \"playerId\":\""+strconv.Itoa(playerId)+"\", \"newHealth\":\""+strconv.Itoa(newHealth)+"\"}")
		case "targetLocked":
			roomId := fmt.Sprintf("%v", message["roomId"])
			broadcastTCP(roomId, string(message_raw))
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

		case "playerDied":
			roomId := fmt.Sprintf("%v", message["roomId"])
			playerId, _ := strconv.Atoi(fmt.Sprintf("%v", message["playerId"]))
			shooterId, _ := strconv.Atoi(fmt.Sprintf("%v", message["shooterId"]))
			//Updating the kills of the shooter
			if killer, ok := rooms[roomId].players[shooterId]; ok {
				if killer.websocket != nil {
					mutex.Lock()
					killer.kills += 1
					fmt.Println("The killer ", shooterId, " has now ", killer.kills, " kills and he needs: ", rooms[roomId].roomRules["killsToWin"], " kills")
					rooms[roomId].players[shooterId] = killer
					//Checking if the room has the rule to win with kills
					if useKills, _ := strconv.ParseBool(rooms[roomId].roomRules["useKills"]); useKills {
						//If it does, checking if the killer has reached the kill Limit
						if killsToWin, _ := strconv.Atoi(rooms[roomId].roomRules["killsToWin"]); killer.kills >= killsToWin {
							//If he reached the limit, informing all the clients about the win/loss
							mutex.Unlock()
							fmt.Println("Someone has won the game")
							broadcastTCP(roomId, "{\"type\":\"GameOver\", \"winnerType\":\"Single\",\"winner\":\""+strconv.Itoa(shooterId)+"\", \"lastKill\":\""+strconv.Itoa(playerId)+"\"}")
							return
						}
					}
					mutex.Unlock()
				}
			}
			if deadPlayer, ok := rooms[roomId].players[playerId]; ok {
				if deadPlayer.websocket != nil {
					mutex.Lock()
					deadPlayer.isDead = true
					rooms[roomId].players[playerId] = deadPlayer
					mutex.Unlock()
					sendTCP(&deadPlayer, "{\"type\":\"playerDied\", \"deadPlayer\":\""+strconv.Itoa(playerId)+"\", \"killer\":\""+strconv.Itoa(shooterId)+"\"}")
					broadcastTCP(roomId, "{\"type\":\"playerDied\", \"deadPlayer\":\""+strconv.Itoa(playerId)+"\", \"killer\":\""+strconv.Itoa(shooterId)+"\"}")
				}
			}
		case "clientDisconnected":
			wasOwner, _ := strconv.ParseBool(fmt.Sprintf("%v", message["wasOwner"]))
			roomId := fmt.Sprintf("%v", message["roomId"])
			disconnectedPlayerId, _ := strconv.Atoi(fmt.Sprintf("%v", message["Id"]))
			disconnectClient(roomId, disconnectedPlayerId)
			if wasOwner {
				newOwner := ""
				for _, v := range rooms[roomId].players {
					if v.websocket != nil {
						newOwner = v.playerId
						break
					}
				}
				broadcastTCP(roomId, "{\"type\":\"transferOwnership\", \"newOwner\":\""+newOwner+"\"}")
			}
		case "completeDelete":
			fmt.Println("A client quit the game")
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
	playersWithoutRoom[playerId] = rooms[roomId].players[playerId]
	delete(rooms[roomId].players, playerId)
	mutex.Unlock()
	//Deleting the room if nobody is in it anymore
	if len(rooms[roomId].players) == 0 {
		mutex.Lock()
		delete(rooms, roomId)
		mutex.Unlock()
		return
	}
}

func updateClientTransforms(roomId string) {
	mutex.Lock()
	transforms := make(map[int]string)
	playersCopy := &rooms[roomId].players
	for k, v := range *playersCopy {
		if len(v.transform) > 1 && v.websocket != nil && !v.isDead {
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

func getOtherClientData(roomId string) string {
	type clientStruct struct {
		Id           string
		Name         string
		Team         string
		PlaneType    string
		PlayerHealth int
		IsReady      bool
	}
	allClientData := []clientStruct{}
	for _, client := range rooms[roomId].players {
		allClientData = append(allClientData, clientStruct{Id: client.playerId, Name: client.name, Team: client.currentTeam, PlaneType: client.planeType, PlayerHealth: client.currentHealth, IsReady: client.isReady})
	}
	result, _ := json.Marshal(allClientData)
	return string(result)
}

/*
func getNamesInRoom(roomId string) []byte {
	var playerNames = map[int]string{}
	mutex.Lock()
	for playerId, player := range rooms[roomId].players {
		playerNames[playerId] = player.name
	}
	mutex.Unlock()
	jsonString, e := json.Marshal(playerNames)
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

func getPlaneTypesInRoom(roomId string) []byte {
	var planeTypes = map[int]string{}
	mutex.Lock()
	for playerId, player := range rooms[roomId].players {
		planeTypes[playerId] = player.planeType
	}
	mutex.Unlock()
	jsonString, e := json.Marshal(planeTypes)
	if e != nil {
		fmt.Println("Something went wrong with getting the names")
		return nil
	}
	return jsonString
}
*/
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

func convertMap(ipt map[string]interface{}) map[string]string {
	out := make(map[string]string)
	for k, v := range ipt {
		out[fmt.Sprintf("%v", k)] = fmt.Sprintf("%v", v)
	}
	return out
}

func handleNewPlayer(conn *websocket.Conn) {
	newId := getNewPlayerId()
	conn.WriteMessage(1, []byte("{\"type\":\"setId\", \"newId\":\""+strconv.Itoa(newId)+"\"}"))
	fmt.Println("Client connected and has now Id: " + strconv.Itoa(newId))
	playersWithoutRoom[newId] = Player{websocket: conn, isNew: true, playerId: strconv.Itoa(newId)}
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
	mutex.Lock()
	defer mutex.Unlock()
	newRoomId := "AAAAAA"
	if _, ok := rooms[newRoomId]; ok {
		return ""
	} else {
		return newRoomId
	}
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
