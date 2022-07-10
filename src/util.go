package main

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"
)

func getRandomRoomId() string {
	mutex.Lock()
	defer mutex.Unlock()
	/*
		newRoomId := "AAAAAA"
		if _, ok := rooms[newRoomId]; ok {
			return ""
		} else {
			return newRoomId
		}
	*/
	//ENABLE AFTER ENTKÄFERUNG
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

func convertMap(ipt map[string]interface{}) map[string]string {
	out := make(map[string]string)
	for k, v := range ipt {
		out[fmt.Sprintf("%v", k)] = fmt.Sprintf("%v", v)
	}
	return out
}

func readFile(file string) []string {
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
	return result
}
