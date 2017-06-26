package main

import (
	"fmt"
	"github.com/milosgajdos83/tenus"
	"github.com/docker/libcontainer/netlink"
	"github.com/docker/docker/pkg/reexec"
	"github.com/vishvananda/netns"
	"runtime"
	"net"
	"os"
	"os/exec"
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
		name: linkName,
		options: tenus.VlanOptions{
			MacAddr: v.NetInterface().HardwareAddr.String(),
			Dev:     linkOptions.Dev,
			Id:      linkOptions.Id,
		},
		link: v,
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

func init() {
	reexec.Register("setup-container-link", reexecSetupContainerLink)
	if reexec.Init() {
		os.Exit(0)
	}
}

func reexecSetupContainerLink() {
	containerName := os.Args[1]
	containerID := os.Args[2]
	parentLink := os.Args[3]
	dockerHost := os.Args[4]
	linkOptions := &tenus.MacVlanOptions{
		Mode:    "bridge",
		MacAddr: os.Args[5],
		Dev:     os.Args[6],
	}

	initializeLogger()
	c:= NewContainer(containerID)

	// Lock OS thread to avoid switching namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save current NS
	origns, err := netns.Get()
	if err != nil {
		c.Logger.Fatalf("Error saving current NS: %s", err.Error())
		os.Exit(1)
	}
	defer origns.Close()

	//Get container PID
	pid, err := tenus.DockerPidByName(containerName, getDockerHostPath(dockerHost))
	if err != nil {
		c.Logger.Fatalf("Error getting container PID: %s", err.Error())
		os.Exit(1)
	}
	c.Logger.Debugf("Container PID is: %v", pid)

	cIfNameTemp := fmt.Sprintf("mcv%v", pid)
	cIfName := linkOptions.Dev

	//Enter container namespace and check if link exists
	if err = tenus.SetNetNsToPid(pid); err != nil {
		c.Logger.Fatalf("Error entering container namespace: %s", err.Error())
		os.Exit(1)
	}
	ns, err := netns.Get()
	if err != nil {
		c.Logger.Fatalf("Error getting container namespace: %s", err.Error())
		os.Exit(1)
	}
	c.Logger.Debugf("Entered container network namespace: %v", ns)

	if _, err := net.InterfaceByName(cIfName); err == nil {
		// Switch back to the original namespace
		netns.Set(origns)

		c.Logger.Warnf("Container link '%s' already exists. Skipping setup.", cIfName)
		os.Exit(0)
	}

	// Switch back to the original namespace
	netns.Set(origns)

	l, err := tenus.NewMacVlanLinkWithOptions(parentLink, tenus.MacVlanOptions{
		Dev:     cIfNameTemp,
		MacAddr: linkOptions.MacAddr,
		Mode:    linkOptions.Mode,
	})
	if err != nil {
		c.Logger.Fatalf("Error creating macvlan link: %s", err.Error())
		os.Exit(1)
	}
	c.Logger.Debugf("MACVLAN link: %s", l)

	//Move link into container namespace
	if err := l.SetLinkNetNsPid(pid); err != nil {
		c.Logger.Fatalf("Error moving link to container namespace: %s", err.Error())
		os.Exit(1)
	}
	c.Logger.Debugf("Moved link '%s' to container", cIfNameTemp)

	//Enter container namespace and rename link
	if err = tenus.SetNetNsToPid(pid); err != nil {
		c.Logger.Fatalf("Error entering container namespace: %s", err.Error())
		os.Exit(1)
	}
	ns, err = netns.Get()
	if err != nil {
		c.Logger.Fatalf("Error getting container namespace: %s", err.Error())
		os.Exit(1)
	}
	c.Logger.Debugf("Entered container network namespace: %v", ns)
	if err = netlink.NetworkChangeName(l.NetInterface(), cIfName); err != nil {
		c.Logger.Fatalf("Error changing interface name: %s", err.Error())
		os.Exit(1)
	}
	c.Logger.Debugf("Renamed link from '%s' to '%s'", cIfNameTemp, cIfName)

	//Bring macvlan interface online
	if err = l.SetLinkUp(); err != nil {
		c.Logger.Fatalf("Error bringing up macvlan interface: %s", err.Error())
		os.Exit(1)
	}
	c.Logger.Debugf("Brought link online: %s", l)

	// Switch back to the original namespace
	netns.Set(origns)

	os.Exit(0)
}

func (c *Container) setupContainerLink(parentLink string, linkOptions tenus.MacVlanOptions, containerName string) (*MacvlanLink, error) {

	cmd := &exec.Cmd{
		Path:   reexec.Self(),
		Args:   append([]string{"setup-container-link"}, containerName, c.ID, parentLink, DockerHost, linkOptions.MacAddr, linkOptions.Dev),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	if err := cmd.Run(); err != nil {
		c.Logger.Fatalf("Setup container link reexec command failed. Command - %s\n", err)
	}

	return &MacvlanLink{
		options: linkOptions,
		name:    linkOptions.Dev,
	}, nil

}
