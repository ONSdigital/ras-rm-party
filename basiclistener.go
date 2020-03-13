package main

import (
	"fmt"

	"github.com/Unleash/unleash-client-go/v3"
)

// BasicListener is a much less noisy version of Unleash's DebugListener
type BasicListener struct{}

// OnError prints out errors
func (l BasicListener) OnError(err error) {
	fmt.Printf("UNLEASH ERROR: %s\n", err.Error())
}

// OnWarning prints out warnings
func (l BasicListener) OnWarning(warning error) {
}

// OnReady prints to the console when the repository is ready
func (l BasicListener) OnReady() {
	fmt.Printf("UNLEASH READY\n")
}

// OnCount prints to the console when the feature is queried
func (l BasicListener) OnCount(name string, enabled bool) {
}

// OnSent prints to the console when the server has uploaded metrics
func (l BasicListener) OnSent(payload unleash.MetricsData) {
}

// OnRegistered prints to the console when the client has registered
func (l BasicListener) OnRegistered(payload unleash.ClientData) {
}
