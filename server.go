package main

import (
	"fmt"
	"log"
	"net/http"
	"syscall"

	"github.com/larytet-go/hashtable"
	"github.com/cespare/xxhash"
	"github.com/larytet-go/unsafepool"
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

func (cp *ControlPanel) start() error {
	log.Printf("Starting sever %s", cp.ipInterface)
	http.HandleFunc("/totals", cp.totals)
	http.HandleFunc("/sensors", cp.totals)
	http.HandleFunc("/exit", cp.exit)
	http.HandleFunc("/", cp.help)

	err := http.ListenAndServe(cp.ipInterface, nil)
	return err
}

type DataPath struct {
	ipInterface string

	// Size of the hashtables is known at the build time
	maxSensorsCount int
	completed       chan struct{}
	exitFlag        bool
	peersStats     *hashtable.Hashtable
	peersIDs       *hashtable.Hashtable
}

type Peer struct {
	// Assumke LAN: IPv4
	ipv4 uint32
	port int
}

func NewPeer(peer *UDPAddr) Peer {
	ipv4 := binary.BigEndian.Uint64(peer.IP[:4])
	return Peer{ipv4, peer.Port}
}

func (p Peer) getID() uint64 {
	peerID := uint64(p.port << 32) + (p.ipv4 << 0)
	return peerID 
}

// Rely on the uniqueness of the IPv4 address in LAN
func (p Peer) getHash() {
	return p.getID() && ((uint64(1) << 32) - 1)
}

func (dp *DataPath) addPeer(peer *UDPAddr) {
	peerID := peerId(peer)
	hashPeerID := hashPeerID(peerID)
	dp.peersStats.Store(peerID, hashPeerID, )
	dp.peersIDs.Store(peerID, hashPeerID, )
}

func (dp *DataPath) processPacket(count int, peer *UDPAddr, buffer []byte) {
	peerID := peerId(peer)
	peerStats, ok, _ := dp.peersStats.Load(peerID, hashPeerID(peerID))
	if !ok {
		peerStats, _ := addPeer(peer)
	}	
}

func (dp *DataPath) start() (error) {
	s, err := net.ResolveUDPAddr("udp4", dp.ipInterface)
	if err != nil {
		return err
	}
	connection, err := net.ListenUDP("udp4", s)
	if err != nil {
		return err
	}
	defer connection.Close()

	buffer := make([]byte, 128)
	for !dp.exitFlag {
		count, peer, err := connection.ReadFromUDP(buffer)
		if err != nil {
			log.Errorf("Read from UDP faild %v", err)
			break // Was the IP intreface restarted?
		}
		dp.processPacket(count, peer, buffer)
	}
	dp.completed <- struct{}{}
	return
}

func main() {
	maxSensorsCount:= 100 * 1024
	maxCollisions := 4
	dp := &DataPath{
		maxSensorsCount: maxSensorsCount,
		completed:       make(chan struct{}),
		peersStats: hashtable.New(maxSensorsCount, maxCollisions),
		peersIDs:     hashtable.New(maxSensorsCount, maxCollisions),
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
