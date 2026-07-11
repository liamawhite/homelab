//go:build darwin

package probe

import (
	"net"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

// lookupNeighbor reports the MAC address for address and whether the entry
// is fully resolved.
func lookupNeighbor(address string) (string, bool) {
	mac, ok := dumpTable()[address]
	return mac, ok
}

// dumpTable reads the BSD routing table, returning every fully-resolved
// IP -> MAC entry.
func dumpTable() map[string]string {
	table := make(map[string]string)

	rib, err := route.FetchRIB(unix.AF_INET, route.RIBTypeRoute, 0)
	if err != nil {
		return table
	}

	msgs, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return table
	}

	for _, m := range msgs {
		rm, ok := m.(*route.RouteMessage)
		if !ok || rm.Flags&unix.RTF_LLINFO == 0 {
			continue
		}
		if len(rm.Addrs) <= unix.RTAX_GATEWAY {
			continue
		}

		dst, ok := rm.Addrs[unix.RTAX_DST].(*route.Inet4Addr)
		if !ok {
			continue
		}

		link, ok := rm.Addrs[unix.RTAX_GATEWAY].(*route.LinkAddr)
		if !ok || len(link.Addr) != 6 {
			continue
		}

		ip := net.IPv4(dst.IP[0], dst.IP[1], dst.IP[2], dst.IP[3]).String()
		table[ip] = net.HardwareAddr(link.Addr).String()
	}

	return table
}
