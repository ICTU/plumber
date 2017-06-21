package main

import (
	"fmt"
	"github.com/milosgajdos83/tenus"
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

	c.Logger.Debugf("Checking if VLAN link '%s' exists", linkOptions.Dev)
	// Check if VLAN link already exists
	if _, err := net.InterfaceByName(linkOptions.Dev); err == nil {
		c.Logger.Printf("VLAN link '%s' already assigned", linkOptions.Dev)
		l, err := getVlanLink(linkOptions.Dev, linkOptions)
		if err != nil {
			c.Logger.Errorf("Failed retrieving VLAN link: %v", err.Error())
			return nil, err
		}
		return l, nil
	}
	// Create VLAN parent interface
	l, err := tenus.NewVlanLinkWithOptions(linkName, linkOptions)
	if err != nil {
		c.Logger.Error(err.Error())
	}
	c.Logger.Debugf("VLAN link: %s", l)
	//Bring interface online
	if err = l.SetLinkUp(); err != nil {
		return nil, err
	}

	c.Logger.Debugf("Brought VLAN link online: %s", l)

	return &VlanLink{
		link:    l,
		name:    linkOptions.Dev,
		options: linkOptions,
	}, nil
}

func (c *Container) setupContainerLink(parentLink string, linkOptions tenus.MacVlanOptions, containerName string) (*MacvlanLink, error) {

	//Get container PID
	pid, err := tenus.DockerPidByName(containerName, getDockerHostPath(DockerHost))
	if err != nil {
		return nil, err
	}
	c.Logger.Debugf("Container PID is: %v", pid)

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
	c.Logger.Debugf("MACVLAN link: %s", l)

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
	c.Logger.Debugf("%s: Moved link '%s' to container", cIfNameTemp)

	//Enter container namespace and rename link
	if err = tenus.SetNetNsToPid(pid); err != nil {
		return nil, err
	}
	c.Logger.Debugf("%s: Entered container network namespace")
	if err = netlink.NetworkChangeName(l.NetInterface(), cIfName); err != nil {
		return nil, err
	}
	c.Logger.Debugf("Renamed link from '%s' to '%s'",cIfNameTemp, cIfName)

	//Bring macvlan interface online
	if err = l.SetLinkUp(); err != nil {
		return nil, err
	}
	c.Logger.Debugf("Brought link online: %s", l)

	// Switch back to the original namespace
	netns.Set(origns)

	return &MacvlanLink{
		options: linkOptions,
		name:    linkOptions.Dev,
		link:    l,
	}, nil
}
