// +build !linux

package main

import "errors"

func (c *Controller) addVIPs(vips []string) error {
	return errors.New("Not support")
}

func (c *Controller) delVIPs(vips []string) error {
	return errors.New("Not support")
}
