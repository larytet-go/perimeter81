package main

import (
	"fmt"
	"log"
	"net/http"
)

type ControlPanel struct {
	ipInterface string
	completed   chan struct{}
}

func (cp *ControlPanel) totals(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "applicaton/json")
	fmt.Fprintf(w, "Totals")
}

// shortcut: for 1M sensors I need a better UI
func (cp *ControlPanel) sensors(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "applicaton/json")
	fmt.Fprintf(w, "sensors")
}

func writeLink(w http.ResponseWriter, ref string) {
	fmt.Fprintf(w, "<br><a href=\"%s\">%s<a>", ref, ref)
}

func (cp *ControlPanel) help(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	links := []string{"sensors", "totals", "exit"}
	for _, link := range links {
		writeLink(w, link)
	}
}

func (cp *ControlPanel) exit(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "Exiting")
	log.Printf("Exiting")
	cp.completed <- struct{}{}
}

func (cp *ControlPanel) start() {
	log.Printf("Starting sever %s", cp.ipInterface)
	http.HandleFunc("/totals", cp.totals)
	http.HandleFunc("/sensors", cp.totals)
	http.HandleFunc("/exit", cp.exit)
	http.HandleFunc("/", cp.help)

	http.ListenAndServe(cp.ipInterface, nil)
}

type DataPath struct {
	// Size of the hashtables is known at the build time
	maxSensorsCount int
	completed       chan struct{}
	exitFlag        bool
}

func (dp *DataPath) start() {
	for !dp.exitFlag {

	}
	dp.completed <- struct{}{}
}

func main() {
	dp := &DataPath{
		maxSensorsCount: 100 * 1024,
		completed:       make(chan struct{}),
	}

	// start data path loop
	go dp.start()

	// start control loop
	cp := &ControlPanel{
		ipInterface: ":8093",
		completed:   make(chan struct{}),
	}
	go cp.start()

	<-cp.completed
	dp.exitFlag = true
	<-dp.completed
}
