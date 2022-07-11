package main

import (
	"strconv"
)

type ErrorMessage struct {
	ErrorText string
}

type ClientConnectedMessage struct {
	Id           int
	Name         string
	Team         string
	IsReady      bool
	PlaneTypes   string
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
	gunName              string
	velocity             string
	planeFacingDirection string
}

//"{\"type\":\"rocketShot\", \"rocketType\":\""+rocketType+"\", \"shooter\":\""+shooter+"\", \"startPos\":"+string(startPosVal)+", \"velocity\":"+string(velocityVal)+", \"facingAngle\":"+string(planeFacingDirectionVal)+", \"targetId\":\""+target+"\"}"

type RocketShotMessage struct {
	rocketType  string
	shooter     string
	gunName     string
	velocity    string
	facingAngle string
	targetId    string
}

func (m RocketShotMessage) getMessageJSON() string {
	return "{\"type\":\"rocketShot\", \"rocketType\":\"" + m.rocketType + "\", \"shooter\":\"" + m.shooter + "\", \"gunName\":\"" + m.gunName + "\", \"velocity\":" + m.velocity + ", \"facingAngle\":" + m.facingAngle + ", \"targetId\":\"" + m.targetId + "\"}"

}

func (m BulletShotMessage) getMessageJSON() string {
	return "{\"type\":\"bulletShot\", \"bulletType\":\"" + m.bulletType + "\", \"shooter\":\"" + m.shooter + "\", \"gunName\":\"" + m.gunName + "\", \"velocity\":" + m.velocity + ", \"planeFacingDirection\":" + m.planeFacingDirection + "}"
}

func (m RejoinMessage) getMessageJSON() string {
	return "{\"type\":\"rejoin\", \"playerId\":\"" + strconv.Itoa(m.playerId) + "\", \"newHealth\":\"" + strconv.Itoa(m.newHealth) + "\"}"
}

func (m JoinSuccessMessage) getMessageJSON() string {
	return "{\"type\":\"joinSuccess\", \"newRoomId\":\"" + m.newRoomId + "\", \"startHealth\":\"" + strconv.Itoa(m.startHealth) + "\", \"sceneIndex\":\"" + m.sceneIndex + "\", \"gameMode\":\"" + m.gameMode + "\", \"otherClients\":" + m.otherClients + "}"
}

func (m CreatedRoomMessage) getMessageJSON() string {
	message := "{\"type\":\"createdRoom\", \"newRoomId\":\"" + m.newRoomId + "\", \"startHealth\":\"" + strconv.Itoa(m.startHealth) + "\", \"sceneIndex\":\"" + m.sceneIndex + "\"}"
	return message
}

func (m ClientConnectedMessage) getMessageJSON() string {
	message := "{\"type\":\"clientConnected\", \"Id\":\"" + strconv.Itoa(m.Id) + "\", \"Name\":\"" + m.Name + "\", \"Team\":\"" + m.Team + "\", \"IsReady\":\"" + strconv.FormatBool(m.IsReady) + "\", \"PlaneTypes\":" + m.PlaneTypes + ", \"PlayerHealth\":\"" + strconv.Itoa(m.PlayerHealth) + "\"}"
	return message
}

func (m ErrorMessage) getMessageJSON() string {
	return "{\"type\":\"Error\", \"value\":\"" + m.ErrorText + "\"}"
}
