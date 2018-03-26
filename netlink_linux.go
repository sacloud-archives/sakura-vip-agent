package main

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

func (c *Controller) addVIPs(vips []string) error {

	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return err
	}

	addrs, err := netlink.AddrList(lo, 4)
	if err != nil {
		return err
	}

	for _, vip := range vips {
		found := false
		strVip := fmt.Sprintf("%s/32 lo", vip)
		for _, ip := range addrs {
			if ip.String() == strVip {
				found = true
				break
			}
		}
		if !found {
			addr, err := netlink.ParseAddr(strVip)
			if err != nil {
				return err
			}
			if err = netlink.AddrAdd(lo, addr); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Controller) delVIPs(vips []string) error {
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return err
	}

	addrs, err := netlink.AddrList(lo, netlink.FAMILY_V4)
	if err != nil {
		return err
	}
	for _, vip := range vips {
		strVip := fmt.Sprintf("%s/32 lo", vip)
		for _, ip := range addrs {
			if ip.String() == strVip {
				if err := netlink.AddrDel(lo, &ip); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
