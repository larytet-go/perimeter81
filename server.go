package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"
)

type ControlPanel struct {
	hostname  string
	completed chan struct{}
	dataPath  *DataPath
}

// sorted array of peers
func getPeers(peersStats map[string](*Accumulator)) []string {
	peers := make([]string, 0, len(peersStats))
	for peer, _ := range peersStats {
		peers = append(peers, peer)
	}
	sort.Strings(peers)
	return peers
}

// shortcut: for 1M sensors I need a better UI
func (cp *ControlPanel) sensorsWeekly(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/plain")

	fmt.Fprintf(w, "%20v %5v %20v %20v %20v (Celcius)\n", "sensor", "days", "weekly max", "weekly min", "weekly average")
	weeklyAverage := float64(0)
	peers := getPeers(cp.dataPath.peersStats)
	for _, peer := range peers {
		stat := cp.dataPath.peersStats[peer]
		result := milliKelvin2CelsiusResult(stat.getResult())
		if !result.nonzero {
			fmt.Fprintf(w, "%20v %20v\n", peer, "not enough data")
			continue
		}
		weeklyAverage += result.windowAverage
		fmt.Fprintf(w, "%20v %5v %20.1f %20.1f %20.1f\n", peer, len(result.average), result.windowMax, result.windowMin, result.windowAverage)
	}
	if len(peers) > 0 {
		fmt.Fprintf(w, "weekly average %0.1f\n", (weeklyAverage / float64(len(peers))))
	}
}

func (cp *ControlPanel) sensorsDaily(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/plain")

	fmt.Fprintf(w, "%20v %5v %20v %20v %20v (Celcius)\n", "sensor", "days", "daily max", "daily min", "daily average")
	for _, peer := range getPeers(cp.dataPath.peersStats) {
		stat := cp.dataPath.peersStats[peer]
		result := milliKelvin2CelsiusResult(stat.getResult())
		if !result.nonzero {
			fmt.Fprintf(w, "%20v %20v\n", peer, "not enough data")
			continue
		}
		fmt.Fprintf(w, "%20v %5v %4.1f %4.1f %4.1f\n", peer, len(result.average), result.max, result.min, result.average)
	}
}

var statistics struct {
	packetsTotal uint64
	startTime    time.Time
}

func (cp *ControlPanel) stats(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/plain")

	elapsed := time.Since(statistics.startTime)
	rate := float64(statistics.packetsTotal) / (float64(elapsed) / float64(time.Second))
	fmt.Fprintf(w, "packetsTotal %20v\nrate %.0f\n", statistics.packetsTotal, rate)
}

func writeLink(w http.ResponseWriter, ref string) {
	fmt.Fprintf(w, "<br><a href=\"%s\">%s<a>", ref, ref)
}

func (cp *ControlPanel) help(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	links := []string{"sensorsweekly", "sensorsdaily", "exit"}
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
	http.HandleFunc("/sensorsweekly", cp.sensorsWeekly)
	http.HandleFunc("/sensorsdaily", cp.sensorsDaily)
	http.HandleFunc("/exit", cp.exit)
	http.HandleFunc("/stats", cp.stats)
	http.HandleFunc("/", cp.help)

	err := http.ListenAndServe(cp.hostname, nil)
	return err
}

type DataPath struct {
	hostname string

	completed chan struct{}
	exitFlag  bool

	// Shortcut: GC will kill this code, should use zero allocation map
	// and reference free Accumulator
	peersStats map[string](*Accumulator)

	// 24 hours
	tickInterval time.Duration

	// 7 days
	days uint64
}

func (dp *DataPath) addPeer(peer *net.UDPAddr) *Accumulator {
	accumulator := NewAccumulator(dp.days)
	dp.peersStats[peer.String()] = accumulator
	// log.Printf("Add peer %v\n", peer)
	return accumulator
}

func (dp *DataPath) processPacket(count int, peer *net.UDPAddr, buffer []byte) {
	statistics.packetsTotal++
	// Kelvin from zero to infinity
	sensorReading := binary.BigEndian.Uint32(buffer[:4])
	// Shortcur: peer.String() is slow. I can do better producing uint64 composition of (ipv4,port)
	peerStats, ok := dp.peersStats[peer.String()]
	if !ok {
		peerStats = dp.addPeer(peer)
	}
	peerStats.Add(uint64(sensorReading))
}

// 24 hours tick
// Shortcut: ignore race condition in the accumulator which can lead to minor errors
// Shortcut: loop over all accumulator creates lot of data caches misses 
// 1M sensors can take a 100ms or so
func (dp *DataPath) tick24h(exitFlag *bool) {
	ticker := time.NewTicker(dp.tickInterval)
	for !(*exitFlag) {
		<-ticker.C
		for _, peerStat := range dp.peersStats {
			peerStat.Tick()
		}
	}
	// Shortcut: It takes time to get to this line, an alternative is `select` and two channels
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
	statistics.startTime = time.Now()
	for !dp.exitFlag {
		count, peer, err := connection.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Read from UDP faild %v", err)
			break // Was the IP intreface restarted?
		}
		dp.processPacket(count, peer, buffer)
	}
	// Shortcut: if there is no connected sensors I will not get here
	log.Printf("Data path exiting\n")
	dp.completed <- struct{}{}
	return nil
}

func boolEnv(env string, defaultValue bool) bool {
	envVar := os.Getenv(env)
	if envVar == "" {
		return defaultValue
	}

	b, err := strconv.ParseBool(envVar)
	if err != nil {
		return defaultValue
	}

	return b
}

func main() {
	hostnameControl := ":8093"
	hostnameData := ":8093"

	modeDemo := boolEnv("MODE_DEMO", false)
	dp := &DataPath{
		hostname:     hostnameData,
		completed:    make(chan struct{}),
		peersStats:   make(map[string](*Accumulator)),
		tickInterval: 24 * time.Hour,
		days:         7,
	}
	if modeDemo {
		log.Println("Demo mode")
		dp.tickInterval = 2 * time.Second
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
		interval:  1 * time.Second,
		completed: make(chan struct{}),
	}
	go sm.start()

	<-cp.completed

	dp.exitFlag, sm.exitFlag = true, true
	<-dp.completed
	<-sm.completed
}
