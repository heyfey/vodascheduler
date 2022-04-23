package main

import (
	"fmt"
	"net"
)

func main() {
	// iprecords, err := net.LookupIP("facebook.com")
	iprecords, err := net.LookupIP("mongodb-svc.voda-scheduler.svc.cluster.local")
	// iprecords, err := net.LookupIP("voda-scheduler.voda-scheduler.svc.cluster.local")
	// iprecords, err := net.LookupIP("mongodb-svc.voda-scheduler")
	// iprecords, err := net.LookupIP("voda-scheduler-rabbitmq.voda-scheduler.svc.cluster.local")
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(len(iprecords))
	for _, ip := range iprecords {
		fmt.Println(ip)
	}
	fmt.Println(iprecords[0])

	// _my-port-name._my-port-protocol.my-svc.my-namespace.svc.cluster-domain.example
	cname, addrs, err := net.LookupSRV("voda-mongodb", "tcp", "mongodb-svc.voda-scheduler.svc.cluster.local")
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(cname)
	fmt.Println(addrs[0].Port)
}
