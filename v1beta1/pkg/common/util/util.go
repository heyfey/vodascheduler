package util

import (
	"net"
)

// GetInClusterServiceIP find in cluster service IP from kube-dns (CoreDNS).
// Domain name pattern: "my-svc.my-namespace.svc.cluster-domain.example".
// Only return the first result.
func GetInClusterServiceIP(service string, namespace string) (net.IP, error) {
	host := service + "." + namespace + ".svc.cluster.local"
	iprecords, err := net.LookupIP(host)
	return iprecords[0], err
}

// GetInClusterServicePort find port of an in cluster service from kube-dns (CoreDNS),
// Domain name pattern: "_my-port-name._my-port-protocol.my-svc.my-namespace.svc.cluster-domain.example".
// The proto is "tcp" or "udp".
// Only return the first result.
func GetInClusterServicePort(service string, namespace string, protocal string, portname string) (uint16, error) {
	domainName := service + "." + namespace + ".svc.cluster.local"
	_, addrs, err := net.LookupSRV(portname, protocal, domainName)
	return addrs[0].Port, err
}
