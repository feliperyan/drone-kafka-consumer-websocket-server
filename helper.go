package main

type tickHelper struct {
	currentTickForAirport map[string]int
	messagesForAirport    map[string][]*droneMessage
}
