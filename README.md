

# The Goal

A sever processing data from the temperature sensors in the building.
The server collects daily and weekly max/min/avg

## Constraints "top down"

* Size of a sensor 5 cm<sup>2</sup>
* 100 floors building, 200 m<sup>2</sup> floor
* Sensors installed on the ceiling and the floor 
* Data packet is 64 bytes Ethernet without preamble and interframe gap
* Packet loss is not critical

Maximum amount of sensors is 800K/floor or 80M sensors in the building.
If all sensors report once every second the sever processes 80M packets/s for a budget of 15 nanos/packet

The server needs 40Gb/s connection. The server has to process the incoming packets at the line rate (C/C++ or V is inevitable)

Calculating simple average, max, min requires collecting of data in a sliding window. 
The server is going to keep 80M sliding windows (7 days X 32 bytes) 250 bytes each for the total of 15GB of memory. 

Time series data base, custom C code, kernel bypass/io_uring: **this is not achievable in 3 hours of coding**

## Constraints "bottom up"

* 0.5GB of RAM, 256B per sensor means the hard limit of 1M sensors, budget 1micro/packet
* Packet loss is not critical
* 64 bytes/packet or 64MB/s or 1Gb/s connection
* 1M packets/s for the time budget 1micro/packet. **Ehernet/UDP will do**
* HTTP vs UPD vs Ethernet. HTTP is buffered - there is a memory page for every socket. I do not see a good package for raw sockets in Go. This leaves UDP.
* Socket ID or IP address/port? Socket ID can save a few cycles in the server, but introduces a requirement of uniqueness on the sensor side. 1M sensors means 20 bits subnet. It can be challending for a DHCP server if I use dynamic IP. Power up of the sensors creates a serious ARP/BOOTP storm.
* Shortcut: Golang GC will kill the server keeping a map of 1M references, but I am doing it anyway. Zero allocation does not fit 3 hours developmemt deadline. Data cache efficient map? Not in 3 hours of coding either.

300 lines in Go? **isn't it too trivial?**

# Software 

## Components

* Lockfree hashtable "Peer IP to sensor stats"
* Up to 1M lock free accumulators
* A single thread processing sensors reports
* A 24h ticker shifting the sliding window every day
* An HTTP server serving reports
* Sensor mock reporting temperature in **milliKelvins** (from 0 to infinity)
* Shortcut: use slower Golang map instead of zero memory allocation hashtable
* Shortcut: ignore race condition between the HTTP server and the DataPath when accessing the accumulators

## Performance 

Sensor mocks and server share the same machine kernel and enjoy back pressure. Every mock is a single thread.
Server's data path is a single thread. 4 mocks and one server hit 600K packets/s. It takes ~2micros to process a packet on a 3.5GHz Ryzen core. This is reasonably close to the stated 1M target. 

Further improvements can come from:

* allocating 7 days accumulators in row instead of columns and reducing data cache misses.
* use a map with a faster lookup
* use int key (sensord ID or IP+port) instead of string in the map

For a large number of sensors hashtable performance is dominated by the data cache miss. 

## Build and run

```sh
go fmt ./... && go build . && MODE_DEMO=true ./perimeter81
```

## Usage tips

```sh
curl http://localhost:8093
while [ 1 ];do echo -en "\\033[0;0H";curl http://localhost:8093/sensorsweekly;sleep 0.2;done;
while [ 1 ];do echo -en "\\033[0;0H";curl http://localhost:8093/sensorsdaily;sleep 0.2;done;
```

## Links

* https://github.com/larytet-go/accumulator/blob/master/accumulator.go
* https://css.bz/2016/12/08/go-raw-sockets.html
* https://github.com/larytet-go/hashtable
* https://gobyexample.com/http-servers
* https://stackoverflow.com/questions/18427655/use-raw-sockets-in-go
* https://www.linode.com/docs/guides/developing-udp-and-tcp-clients-and-servers-in-go/
* https://github.com/larytet-go/unsafepool
* https://developers.mattermost.com/blog/hands-on-iouring-go/  - io_uring
* https://github.com/google/gvisor  - user space Linux kernel including TCP/IP stack
* https://github.com/vlang/v/blob/master/examples/concurrency/concurrency.v  - C looking like Go

Some caches

* https://github.com/patrickmn/go-cache
* https://github.com/allegro/bigcache - memory management
* https://github.com/coocood/freecache  - memory management
* https://github.com/koding/cache
* https://github.com/sch00lb0y/vegamcache
* https://github.com/OneOfOne/cmap
* https://github.com/larytet/mcachego - memory management
* https://github.com/golang/groupcache
* https://github.com/capnproto/capnproto/blob/master/c++/src/kj/map.h