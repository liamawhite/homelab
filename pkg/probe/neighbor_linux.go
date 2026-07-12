//go:build linux

package probe

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

const arpFlagComplete = 0x2

// lookupNeighbor reports the MAC address for address and whether the entry
// is fully resolved.
func lookupNeighbor(address string) (string, bool) {
	mac, ok := dumpTable()[address]
	return mac, ok
}

// dumpTable reads /proc/net/arp, returning every fully-resolved IP -> MAC
// entry.
func dumpTable() map[string]string {
	table := make(map[string]string)

	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return table
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan() // skip header line

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}

		mac := fields[3]
		if mac == "00:00:00:00:00:00" {
			continue
		}

		flags, err := strconv.ParseInt(strings.TrimPrefix(fields[2], "0x"), 16, 64)
		if err != nil || flags&arpFlagComplete == 0 {
			continue
		}

		table[fields[0]] = mac
	}

	return table
}
