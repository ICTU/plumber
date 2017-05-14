package main

import (
	"fmt"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/milosgajdos83/tenus"
)

type Container struct {
	ID string
}

type ContainerNetwork struct {
	NetworkMode   string
	VlanID        string
	IP            string
	Gateway       string
	InterfaceName string
	MAC           string
}

func (c *Container) setupNetwork(containerName string, cn *ContainerNetwork) {
	switch cn.NetworkMode {
	case "macvlan":
		logrus.Printf("%s: Setting up '%s' network for container '%s'", c.ID, cn.NetworkMode, containerName)
		c.setupMacvlanNetwork(containerName, cn)
	default:
		logrus.Printf("%s: I do not know how to setup '%s' network", c.ID, cn.NetworkMode)
	}
}

func (c *Container) setupMacvlanNetwork(containerName string, cn *ContainerNetwork) {
	parentLinkName := HostLinkName
	if cn.VlanID != "" {
		vlanID, _ := strconv.ParseUint(cn.VlanID, 0, 64)
		parentLink, err := c.setupHostLink(HostLinkName, tenus.VlanOptions{
			MacAddr: generateMAC(),
			Dev:     fmt.Sprintf("%s.%d", HostLinkName, vlanID),
			Id:      uint16(vlanID),
		})
		if err != nil {
			logrus.Fatalf("%s: Failed setting up parent link: %v", c.ID, err.Error())
		}
		logrus.Printf("%s: Parent link online: %v", c.ID, parentLink.options.MacAddr)
		parentLinkName = parentLink.name
	}

	containerLink, err := c.setupContainerLink(parentLinkName, tenus.MacVlanOptions{
		Dev:     HostLinkName,
		MacAddr: generateMAC(),
		Mode:    "bridge",
	}, containerName)
	if err != nil {
		logrus.Fatalf("%s: Failed setting up container link: %v", c.ID, err.Error())
	}
	logrus.Printf("%s: Container link online: %v", c.ID, containerLink.options.MacAddr)
	logrus.Debugf("Container link info: %v", containerLink)
}

func (c *Container) handleContainerEvent(d *docker.Client, event *docker.APIEvents) {

	containerName := event.Actor.Attributes["name"]
	containerID := event.Actor.ID

	switch event.Action {
	case "start":
		containerInfo, err := containerInfo(d, containerID)
		if err != nil {
			logrus.Errorf("%s: Error inspecting container: %s", c.ID, err.Error())
		}
		if containerInfo != nil {
			cn := ContainerNetwork{
				NetworkMode:   containerInfo.Config.Labels["plumber.network.mode"],
				VlanID:        containerInfo.Config.Labels["plumber.network.vlanid"],
				Gateway:       containerInfo.Config.Labels["plumber.network.gateway"],
				InterfaceName: containerInfo.Config.Labels["plumber.network.interfacename"],
			}

			if cn.NetworkMode != "" {
				logrus.Printf("%s: Container '%s' event -> '%s'", c.ID, containerName, event.Action)
				c.setupNetwork(containerName, &cn)
			}
		}
	}
}
