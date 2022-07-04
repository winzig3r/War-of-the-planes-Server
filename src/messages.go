package main

import "strconv"

type ErrorMessage struct {
	ErrorText string
}

type ClientConnectedMessage struct {
	Id           int
	Name         string
	Team         string
	IsReady      bool
	PlaneType    string
	PlayerHealth int
}

type CreatedRoomMessage struct {
	newRoomId   string
	startHealth int
	sceneIndex  string
}

type JoinSuccessMessage struct {
	newRoomId    string
	startHealth  int
	sceneIndex   string
	gameMode     string
	otherClients string
}

type RejoinMessage struct {
	playerId  int
	newHealth int
}

type BulletShotMessage struct {
	bulletType           string
	shooter              string
	startPos             string
	velocity             string
	planeFacingDirection string
}

//"{\"type\":\"rocketShot\", \"rocketType\":\""+rocketType+"\", \"shooter\":\""+shooter+"\", \"startPos\":"+string(startPosVal)+", \"velocity\":"+string(velocityVal)+", \"facingAngle\":"+string(planeFacingDirectionVal)+", \"targetId\":\""+target+"\"}"

type RocketShotMessage struct {
	rocketType  string
	shooter     string
	startPos    string
	velocity    string
	facingAngle string
	targetId    string
}

type Message interface {
	getMessageJSON() string
}

func (m RocketShotMessage) getMessageJSON() string {
	return "{\"type\":\"rocketShot\", \"rocketType\":\"" + m.rocketType + "\", \"shooter\":\"" + m.shooter + "\", \"startPos\":" + m.startPos + ", \"velocity\":" + m.velocity + ", \"facingAngle\":" + m.facingAngle + ", \"targetId\":\"" + m.targetId + "\"}"

}

func (m BulletShotMessage) getMessageJSON() string {
	return "{\"type\":\"bulletShot\", \"bulletType\":\"" + m.bulletType + "\", \"shooter\":\"" + m.shooter + "\", \"startPos\":" + m.startPos + ", \"velocity\":" + m.velocity + ", \"planeFacingDirection\":" + m.planeFacingDirection + "}"
}

func (m RejoinMessage) getMessageJSON() string {
	return "{\"type\":\"rejoin\", \"playerId\":\"" + strconv.Itoa(m.playerId) + "\", \"newHealth\":\"" + strconv.Itoa(m.newHealth) + "\"}"
}

func (m JoinSuccessMessage) getMessageJSON() string {
	return "{\"type\":\"joinSuccess\", \"newRoomId\":\"" + m.newRoomId + "\", \"startHealth\":\"" + strconv.Itoa(m.startHealth) + "\", \"sceneIndex\":\"" + m.sceneIndex + "\", \"gameMode\":\"" + m.gameMode + "\", \"otherClients\":" + m.otherClients + "}"
}

func (m CreatedRoomMessage) getMessageJSON() string {
	return "{\"type\":\"createdRoom\", \"newRoomId\":\"" + m.newRoomId + "\", \"startHealth\":\"" + strconv.Itoa(m.startHealth) + "\", \"sceneIndex\":\"" + m.sceneIndex + "\"}"
}

func (m ClientConnectedMessage) getMessageJSON() string {
	return "{\"type\":\"clientConnected\", \"Id\":\"" + strconv.Itoa(m.Id) + "\", \"Name\":\"" + m.Name + "\", \"Team\":\"" + m.Team + "\", \"IsReady\":\"" + strconv.FormatBool(m.IsReady) + "\", \"PlaneType\":\"" + m.PlaneType + "\", \"PlayerHealth\":\"" + strconv.Itoa(m.PlayerHealth) + "\"}"
}

func (m ErrorMessage) getMessageJSON() string {
	return "{\"type\":\"Error\", \"value\":\"" + m.ErrorText + "\"}"
}

func getMessage(m Message) string {
	return m.getMessageJSON()
}
