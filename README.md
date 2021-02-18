

# The Goal

A sever processing data from the temperature sensors in the building.
The server collects daily and weekly max/min/avg

## Assumptions I

* Size of a sensor 5 cm<sup>2</sup>
* 100 floors building, 200 m<sup>2</sup> floor
* Sensors installed on the ceiling and the floor 
* Data packet is 64 bytes Ethernet without preamble and interframe gap

Maximum amount of sensors is 800K/floor or 80M sensors in the building.
If all sensors report once every second the sever processes 80M packets/s for a budget of 10 micros/packet

The server needs 40Gb/s connection. The server has to process the incoming packets at the line rate 

Calculating simple average, max, min requires collecting of data in a sliding window. Every sensor can produce up to 600K events/week. The server is going to keep 80M sliding windows 600KB each for the total of 48TB of memory. 

Time series data base, custom C code: **this is not achievable in 3 hours of coding**

## Assumptions II

* 1TB of RAM, 600KB per sensor means the hard limit of 1M sensors
* 1M packets/s for the time budget 1ms/packet Ehernet/UDP will do
* 64 bytes/packet or 64MB/s or 1Gb/s connection

300 lines in Go? **isn't it too trivial?**

# Software components

* Lockfree hashtable "Ehernet address to sensor index"
* Lockfree hashtable "sensor index to Eternet address"
* 600K lock free accumulators
* A single thread processing senors reports
* A (single hread?) HTTP server for UI

## Links

* https://github.com/larytet-go/accumulator/blob/master/accumulator.go
* https://css.bz/2016/12/08/go-raw-sockets.html
* https://github.com/larytet-go/hashtable