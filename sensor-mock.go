package main

import (
	"encoding/binary"
	"log"
	"math"
	"math/rand"
	"net"
	"time"
)

func celsiusToMilliKelvins(v float64) int {
	return int(math.Trunc(1000 * (v + 273.15)))
}

type SensorMock struct {
	hostname  string
	sensors   int
	interval  time.Duration
	completed chan struct{}
	exitFlag  bool
}

func (sm *SensorMock) start() error {
	log.Printf("Mock resolve %s\n", sm.hostname)
	s, err := net.ResolveUDPAddr("udp4", sm.hostname)
	if err != nil {
		log.Printf("Failed to resolve %s %v", sm.hostname, err)
		return err
	}
	connections := []*net.UDPConn{}
	for i := 0; i < sm.sensors; i++ {

		log.Printf("Mock dial %s\n", sm.hostname)
		c, err := net.DialUDP("udp4", nil, s)
		if err != nil {
			log.Printf("Failed to dial %s %v", sm.hostname, err)
			return err
		}
		connections = append(connections, c)
		defer c.Close()
	}

	log.Printf("Mock sending data %s\n", sm.hostname)
	for !sm.exitFlag {
		time.Sleep(sm.interval)
		temperature := celsiusToMilliKelvins(float64(rand.Intn(70))) // -273C to +70C
		for _, connection := range connections {
			data := make([]byte, 4)
			binary.BigEndian.PutUint32(data, uint32(temperature))
			count, err := connection.Write(data)
			if err != nil || count != len(data) {
				log.Printf("Mock send failed %d %v\n", count, err)
			}
			temperature += 1 // all sensors are different
		}
		// log.Printf("Mock sending data %s %d\n", sm.hostname, temperature)
	}

	log.Printf("Mock exiting\n")
	sm.completed <- struct{}{}
	return nil
}
