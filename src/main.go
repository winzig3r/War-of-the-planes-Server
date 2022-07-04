package main

var names []string

func main() {
	names = readFile(namesFileLocation)
	go startTCP()
	startUDP()
}
