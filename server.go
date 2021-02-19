package main

import (
	"fmt"
	"log"
    "net/http"
)


type ControlPanel struct {
	port int
}

func (cp *ControlPanel) totals(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "Totals")
}

// shortcut: for 1M sensors I need a better UI
func (cp *ControlPanel) sensors(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "sensors")
}

func (cp *ControlPanel) start() {
	http.HandleFunc("/totals", cp.totals)	
	http.HandleFunc("/sensors", cp.totals)	
	
	http.ListenAndServe(cp.port, nil)
}



func dataPath() {

}



func main() {
	// start data path loop 
	go dataPath()
	// start control loop
	go controlPanel()
}