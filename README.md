

# The Goal

A sever processing data from the temperature sensors in the building.
The server collects max/min/avg

# Assumptions

* Size of a sensor 5 cm<sup>2</sup>
* 100 floors building, 200 m<sup>2</sup> floor
* Sensors installed on the ceiling and the floor 
* Data packet is 64 bytes Ethernet wihout preamble and interframe gap

Maimum amount of sensors is 800K/floor or 80M sensors in the building.
If all sensors report once every second the sever processes 80M packets/s for a budget of 10 micros/packet

The server needs 10Gb/s connection. The server has to process the incoming packets at the line rate 


