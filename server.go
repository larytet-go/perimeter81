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
func (cp *ControlPanel) sensorsWeekly(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/plain")

	fmt.Fprintf(w, "%20v %20v %20v %20v %20v\n", "sensor", "days", "weekly max", "weekly min", "weekly average")
	for peer, stat := range cp.dataPath.peersStats {
		result := stat.getResult()
		if !result.nonzero {
			fmt.Fprintf(w, "%20v %20v\n", peer, "not enough data")
			continue
		}
		fmt.Fprintf(w, "%20v %20v %20v %20v %20v\n", peer, len(result.average), result.windowMax, result.windowMin, result.windowAverage)
	}
}

func (cp *ControlPanel) sensorsDaily(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/plain")

	fmt.Fprintf(w, "%20v %20v %20v %20v %20v\n", "sensor", "days", "daily max", "daily min", "daily average")
	for peer, stat := range cp.dataPath.peersStats {
		result := stat.getResult()
		if !result.nonzero {
			fmt.Fprintf(w, "%20v %20v\n", peer, "not enough data")
			continue
		}
		fmt.Fprintf(w, "%20v %20v %v %v %v\n", peer, len(result.average), result.max, result.min, result.average)
	}
}

func writeLink(w http.ResponseWriter, ref string) {
	fmt.Fprintf(w, "<br><a href=\"%s\">%s<a>", ref, ref)
}

func (cp *ControlPanel) help(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	links := []string{"sensorsweekly", "sensorsdaily", "totals", "exit"}
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
	http.HandleFunc("/sensorsweekly", cp.sensorsWeekly)
	http.HandleFunc("/sensorsdaily", cp.sensorsDaily)
	http.HandleFunc("/exit", cp.exit)
	http.HandleFunc("/", cp.help)

	err := http.ListenAndServe(cp.hostname, nil)
	return err
}

type DataPath struct {
	hostname string

	completed chan struct{}
	exitFlag  bool

	// Shortcut: GC will kill this code, should use zero allocation map
	peersStats map[string](*Accumulator)

	// 24 hours
	tickInterval time.Duration
}

func (dp *DataPath) addPeer(peer *net.UDPAddr) *Accumulator {
	accumulator := NewAccumulator()
	dp.peersStats[peer.String()] = accumulator
	// log.Printf("Add peer %v\n", peer)
	return accumulator
}

func (dp *DataPath) processPacket(count int, peer *net.UDPAddr, buffer []byte) {
	// Kelvin from zero to infinity
	sensorReading := binary.BigEndian.Uint32(buffer[:4])
	// log.Printf("Got data from UDP %v %d", peer, sensorReading)
	peerStats, ok := dp.peersStats[peer.String()]
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
		peersStats:   make(map[string](*Accumulator)),
		tickInterval: 2 * time.Second, // Use time.Day (24 hours) in the real system
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
		sensors:   40,
		interval:  10 * time.Millisecond,
		completed: make(chan struct{}),
	}
	go sm.start()

	<-cp.completed

	dp.exitFlag, sm.exitFlag = true, true
	<-dp.completed
	<-sm.completed
}
