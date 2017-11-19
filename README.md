# ZKPacket 

## Summary
Zookeeper Packet (ZKPacket) has a goal to be a low overhead packet sniffer to collect high fidelity metrics and logs on a particular Zookeeper service. Operating Zookeeper is not a small cost for any team and one issue in this space is the lack of higher fidelity metrics in the Zookeeper program itself. 

The style of collecting the stas from the four letter commands is fine for operational alarms but falls short when trying to gain higher fidelity metrics and information of what is happening on the cluster. 

## Building
The supplied make file will build and test the codebase. 
```
make 
```

## TODO list
* [] Setup crossdocker tests with Zookeeper 3.4 and 3.5-alpha
* [] More tests. 
* [] Orginize and trim the package structure and public APIs.
* [] Docker compose framework for more testing of real traffic. 