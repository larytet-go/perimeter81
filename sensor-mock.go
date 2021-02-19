package main

import (
	"log"
	"math"
	"math/rand"
	"net"
	"strconv"
	"time"
)

func celsiusToMilliKelvins(v float64) int {
	return int(math.Trunc(1000 * (v + 273.15)))
}

func start(hostname string, sensors int, interval time.Duration) {
	s, err := net.ResolveUDPAddr("udp4", hostname)
	if err != nill {
		log.Printf("Failed to resolve %s %v", hostname, err)
		return
	}

	c, err := net.DialUDP("udp4", nil, s)
	if err != nill {
		log.Printf("Failed to dial %s %v", hostname, err)
		return
	}
	defer c.Close()

	for {
		time.Sleep(interval)
		temperature := celsiusToMilliKelvins(rand.Intn(70)) // -273C to +70C
		for i := 0; i < sensors; i++ {
			data := []byte(strconv.Itoa(temperature))
			c.WriteToUDP(data, s)
			temperature += 1 // all sensors are different
		}
	}
}
