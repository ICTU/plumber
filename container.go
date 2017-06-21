package main

import (
	"fmt"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/milosgajdos83/tenus"
)

type Container struct {
	ID     string
	Logger *logrus.Entry
}

type ContainerNetwork struct {
	NetworkMode   string
	VlanID        string
	IP            string
	Gateway       string
	InterfaceName string
	MAC           string
}

func NewContainer(event *docker.APIEvents) *Container {
	id := event.Actor.ID[0:12]
	logEntry := Logger.WithFields(logrus.Fields{"ID": id})
	return &Container{
		ID:     id,
		Logger: logEntry,
	}
}

func getContainerNetwork(containerInfo *docker.Container) ContainerNetwork {
	return ContainerNetwork{
		NetworkMode:   containerInfo.Config.Labels["plumber.network.mode"],
		VlanID:        containerInfo.Config.Labels["plumber.network.vlanid"],
		Gateway:       containerInfo.Config.Labels["plumber.network.gateway"],
		InterfaceName: containerInfo.Config.Labels["plumber.network.interfacename"],
	}
}

func (c *Container) setupNetwork(containerName string, cn *ContainerNetwork) {
	switch cn.NetworkMode {
	case "macvlan":
		c.Logger.Printf("Setting up '%s' network for container '%s'", cn.NetworkMode, containerName)
		c.setupMacvlanNetwork(containerName, cn)
	default:
		c.Logger.Printf("I do not know how to setup '%s' network", cn.NetworkMode)
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
			c.Logger.Fatalf("Failed setting up parent link: %v", err.Error())
		}
		c.Logger.Printf("Parent link online: %v", parentLink.options.MacAddr)
		parentLinkName = parentLink.name
	}

	containerLink, err := c.setupContainerLink(parentLinkName, tenus.MacVlanOptions{
		Dev:     HostLinkName,
		MacAddr: generateMAC(),
		Mode:    "bridge",
	}, containerName)
	if err != nil {
		c.Logger.Fatalf("Failed setting up container link: %v", err.Error())
	}
	c.Logger.Printf("Container link online: %v", containerLink.options.MacAddr)
	c.Logger.Debugf("Container link info: %v", containerLink)
}

func (c *Container) handleContainerEvent(d *docker.Client, event *docker.APIEvents) {

	containerName := event.Actor.Attributes["name"]
	containerID := event.Actor.ID

	switch event.Action {
	case "start":
		containerInfo, err := containerInfo(d, containerID)
		if err != nil {
			c.Logger.Errorf("Error inspecting container: %s", err.Error())
		}
		if containerInfo != nil {
			cn := getContainerNetwork(containerInfo)

			if cn.NetworkMode != "" {
				c.Logger.Printf("Container '%s' event -> '%s'", containerName, event.Action)
				c.setupNetwork(containerName, &cn)
			}
		}
	}
}
