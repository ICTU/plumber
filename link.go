package main

import (
	"fmt"
	"github.com/milosgajdos83/tenus"
	"github.com/Sirupsen/logrus"
	"github.com/docker/libcontainer/netlink"
	"github.com/vishvananda/netns"
	"net"
	"runtime"
)

type VlanLink struct {
	name    string
	options tenus.VlanOptions
	link    tenus.Linker
}

type MacvlanLink struct {
	name    string
	options tenus.MacVlanOptions
	link    tenus.Linker
}

func getVlanLink(linkName string, linkOptions tenus.VlanOptions) (*VlanLink, error) {
	v, err := tenus.NewLinkFrom(linkName)
	if err != nil {
		return nil, err
	}

	return &VlanLink{
		name:    linkName,
		options: tenus.VlanOptions{
			MacAddr: v.NetInterface().HardwareAddr.String(),
			Dev: linkOptions.Dev,
			Id: linkOptions.Id,
		},
		link:    v,
	}, nil

}

func (c *Container) setupHostLink(linkName string, linkOptions tenus.VlanOptions) (*VlanLink, error) {

	logrus.Debugf("%s: Checking if VLAN link '%s' exists", c.ID, linkOptions.Dev)
	// Check if VLAN link already exists
	if _, err := net.InterfaceByName(linkOptions.Dev); err == nil {
		logrus.Printf("%s: VLAN link '%s' already assigned", c.ID, linkOptions.Dev)
		l, err := getVlanLink(linkOptions.Dev, linkOptions)
		if err != nil {
			logrus.Errorf("%s: Failed retrieving VLAN link: %v", c.ID, err.Error())
			return nil, err
		}
		return l, nil
	}
	// Create VLAN parent interface
	l, err := tenus.NewVlanLinkWithOptions(linkName, linkOptions)
	if err != nil {
		logrus.Error(err.Error())
	}
	logrus.Debugf("%s: VLAN link: %s", c.ID, l)
	//Bring interface online
	if err = l.SetLinkUp(); err != nil {
		return nil, err
	}

	logrus.Debugf("%s: Brought VLAN link online: %s", c.ID, l)

	return &VlanLink{
		link:    l,
		name:    linkOptions.Dev,
		options: linkOptions,
	}, nil
}

func (c *Container) setupContainerLink(parentLink string, linkOptions tenus.MacVlanOptions, containerName string) (*MacvlanLink, error) {

	//Get container PID
	pid, err := tenus.DockerPidByName(containerName, DockerSocketPath)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("%s: Container PID is: %v", c.ID, pid)

	cIfNameTemp := fmt.Sprintf("mcv%v", pid)
	cIfName := linkOptions.Dev
	l, err := tenus.NewMacVlanLinkWithOptions(parentLink, tenus.MacVlanOptions{
		Dev:     cIfNameTemp,
		MacAddr: linkOptions.MacAddr,
		Mode:    linkOptions.Mode,
	})
	if err != nil {
		return nil, err
	}
	logrus.Debugf("%s: MACVLAN link: %s", c.ID, l)

	// Lock OS thread to avoid switching namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save current NS
	origns, _ := netns.Get()
	defer origns.Close()

	//Move link into container namespace
	if err := l.SetLinkNetNsPid(pid); err != nil {
		return nil, err
	}
	logrus.Debugf("%s: Moved link '%s' to container", c.ID, cIfNameTemp)

	//Enter container namespace and rename link
	if err = tenus.SetNetNsToPid(pid); err != nil {
		return nil, err
	}
	logrus.Debugf("%s: Entered container network namespace", c.ID)
	if err = netlink.NetworkChangeName(l.NetInterface(), cIfName); err != nil {
		return nil, err
	}
	logrus.Debugf("%s: Renamed link from '%s' to '%s'", c.ID, cIfNameTemp, cIfName)

	//Bring macvlan interface online
	if err = l.SetLinkUp(); err != nil {
		return nil, err
	}
	logrus.Debugf("%s: Brought link online: %s", c.ID, l)

	// Switch back to the original namespace
	netns.Set(origns)

	return &MacvlanLink{
		options: linkOptions,
		name:    linkOptions.Dev,
		link:    l,
	}, nil
}
