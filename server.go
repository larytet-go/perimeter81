package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

type ControlPanel struct {
	hostname  string
	completed chan struct{}
	dataPath  *DataPath
}

func (cp *ControlPanel) totals(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "applicaton/json")
	fmt.Fprintf(w, "Totals")
}

// shortcut: for 1M sensors I need a better UI
func (cp *ControlPanel) sensors(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/plain")

	for peer, stat := range cp.dataPath.peersStats {
		result := stat.getResult(true)
		fmt.Fprintf(w, "%v daily averages %v, max %d, min %d\n", peer, result.results, result.max, result.min)
	}
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

func (cp *ControlPanel) start() error {
	log.Printf("Starting sever %s", cp.hostname)
	http.HandleFunc("/totals", cp.totals)
	http.HandleFunc("/sensors", cp.sensors)
	http.HandleFunc("/exit", cp.exit)
	http.HandleFunc("/", cp.help)

	err := http.ListenAndServe(cp.hostname, nil)
	return err
}

type DataPath struct {
	hostname string

	completed chan struct{}
	exitFlag  bool

	// Shortcut: GC will kill this code, should use github.com/larytet-go/hashtable instead
	peersStats map[*net.UDPAddr](*Accumulator)

	// 24 hours
	tickInterval time.Duration
}

func (dp *DataPath) addPeer(peer *net.UDPAddr) *Accumulator {
	accumulator := NewAccumulator()
	dp.peersStats[peer] = accumulator
	return accumulator
}

func (dp *DataPath) processPacket(count int, peer *net.UDPAddr, buffer []byte) {
	// Kelvin from zero to infinity
	sensorReading := binary.BigEndian.Uint32(buffer[:4])
	// log.Printf("Got data from UDP %v %d", peer, sensorReading)
	peerStats, ok := dp.peersStats[peer]
	if !ok {
		peerStats = dp.addPeer(peer)
	}
	peerStats.Add(uint64(sensorReading))
}

// 24 hours tick
// Shortcut: ignore race condition in the accumulator
// Shortcut: loop over all accumulator can take time
func (dp *DataPath) tick24h(exitFlag *bool) {
	ticker := time.NewTicker(dp.tickInterval)
	for !(*exitFlag) {
		<-ticker.C
		for _, peerStat := range dp.peersStats {
			peerStat.Tick()
		}
	}
	log.Printf("24h ticker exiting\n")
}

func (dp *DataPath) start() error {
	log.Printf("Data path resolve %s\n", dp.hostname)
	s, err := net.ResolveUDPAddr("udp4", dp.hostname)
	if err != nil {
		return err
	}

	log.Printf("Data path listen %s\n", dp.hostname)
	connection, err := net.ListenUDP("udp4", s)
	if err != nil {
		return err
	}
	defer connection.Close()
	go dp.tick24h(&dp.exitFlag)

	log.Printf("Data path enter loop %s\n", dp.hostname)
	buffer := make([]byte, 128)
	for !dp.exitFlag {
		count, peer, err := connection.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Read from UDP faild %v", err)
			break // Was the IP intreface restarted?
		}
		dp.processPacket(count, peer, buffer)
	}
	log.Printf("Data path exiting\n")
	dp.completed <- struct{}{}
	return nil
}

func main() {
	hostnameControl := ":8093"
	hostnameData := ":8094"

	dp := &DataPath{
		hostname:     hostnameData,
		completed:    make(chan struct{}),
		peersStats:   make(map[*net.UDPAddr](*Accumulator)),
		tickInterval: 20 * time.Second, // 24 * time.Hour
	}

	// start data path loop
	go dp.start()

	// start control loop
	cp := &ControlPanel{
		hostname:  hostnameControl,
		completed: make(chan struct{}),
		dataPath:  dp,
	}
	go cp.start()

	sm := &SensorMock{
		hostname:  hostnameData,
		sensors:   2,
		interval:  time.Second,
		completed: make(chan struct{}),
	}
	go sm.start()

	<-cp.completed

	dp.exitFlag, sm.exitFlag = true, true
	<-dp.completed
	<-sm.completed
}
