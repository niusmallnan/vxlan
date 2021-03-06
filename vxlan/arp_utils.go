package vxlan

import (
	"net"

	"github.com/pkg/errors"
	"github.com/rancher/log"
	"github.com/vishvananda/netlink"
)

func getCurrentARPEntries(link netlink.Link, ipnets []*net.IPNet) (map[string]*netlink.Neigh, error) {
	neighs, err := netlink.NeighList(link.Attrs().Index, netlink.FAMILY_V4)
	if err != nil {
		log.Errorf("Failed to getCurrentARPEntries, NeighList: %v", err)
		return nil, err
	}

	arpEntries := make(map[string]*netlink.Neigh)
	for index, n := range neighs {
		for _, ipnet := range ipnets {
			if ipnet.Contains(n.IP.To4()) {
				log.Debugf("getCurrentARPEntries: Neigh %+v", n)
				arpEntries[n.IP.To4().String()] = &neighs[index]
			}
		}
	}

	log.Debugf("getCurrentARPEntries: arpEntries %v", arpEntries)
	return arpEntries, err
}

func getDesiredARPEntries(link netlink.Link, arp map[string]net.HardwareAddr) map[string]*netlink.Neigh {
	arpEntries := make(map[string]*netlink.Neigh)

	for ip, mac := range arp {
		n := &netlink.Neigh{
			IP:           net.ParseIP(ip),
			HardwareAddr: mac,
			LinkIndex:    link.Attrs().Index,
			State:        netlink.NUD_PERMANENT,
			Flags:        netlink.NTF_SELF,
		}
		arpEntries[ip] = n
	}
	log.Debugf("getDesiredARPEntries: %v", arpEntries)
	return arpEntries
}

func updateARP(oldEntries map[string]*netlink.Neigh, newEntries map[string]*netlink.Neigh) error {
	var e error

	for ip, oe := range oldEntries {
		ne, ok := newEntries[ip]
		if ok {
			if ne.HardwareAddr.String() != oe.HardwareAddr.String() {
				log.Debugf("updateARP: new entry mac: %s, ip: %s", ne.HardwareAddr.String(), ip)
				log.Debugf("updateARP: old entry mac: %s, ip: %s", oe.HardwareAddr.String(), ip)
				err := netlink.NeighSet(ne)
				if err != nil {
					log.Errorf("updateARP: failed to NeighSet,  %v, %+v", err, ne)
					e = errors.Wrap(e, err.Error())
				}
			}
			delete(newEntries, ip)
		} else {
			log.Debugf("updateARP: delete invalid arp entry: %+v", oe)
			err := netlink.NeighDel(oe)
			if err != nil {
				log.Errorf("updateARP: failed to NeighDel not in newEntries, %v", err)
				e = errors.Wrap(e, err.Error())
			}
		}
	}

	for ip, ne := range newEntries {
		err := netlink.NeighAdd(ne)
		if err != nil {
			log.Errorf("updateARP: failed to NeighAdd, %v, %s", err, ip)
			e = errors.Wrap(e, err.Error())
		}
	}

	return e
}
