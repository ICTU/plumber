package main

import (
	"fmt"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/milosgajdos83/tenus"
	"regexp"
)

type Container struct {
	ID     string
	Name   string
	Logger *logrus.Entry
}

type ContainerNetworkConfig struct {
	NetworkMode string
	VlanID      string
}

func NewContainer(id string) *Container {
	logEntry := Logger.WithFields(logrus.Fields{"ID": id})
	return &Container{
		ID:     id,
		Logger: logEntry,
	}
}

func (c *Container) getContainerNetworkConfig(containerInfo *docker.Container) ContainerNetworkConfig {
	// Check if pipework command is passed as environment variable.
	pattern := regexp.MustCompile(`((\w*_)*pipework_cmd(_\w*)*=(.*))`)
	for _, env := range containerInfo.Config.Env {
		if pattern.MatchString(env) {
			pipeworkCMD := pattern.FindStringSubmatch(env)[4]
			c.Logger.Debugf("Pipework CMD: %s", pipeworkCMD)
			pattern = regexp.MustCompile(`^(\w*)( -i (\w*))? @CONTAINER_NAME@ (\S*)( @(\d+))?$`)
			return ContainerNetworkConfig{
				NetworkMode: "macvlan",
				VlanID:      pattern.FindStringSubmatch(pipeworkCMD)[6],
			}
		}
	}

	return ContainerNetworkConfig{
		NetworkMode: containerInfo.Config.Labels["plumber.network.mode"],
		VlanID:      containerInfo.Config.Labels["plumber.network.vlanid"],
	}
}

func (c *Container) setupNetwork(containerName string, cn *ContainerNetworkConfig) {
	switch cn.NetworkMode {
	case "macvlan":
		c.Logger.Printf("Setting up '%s' network for container '%s'", cn.NetworkMode, containerName)
		c.setupMacvlanNetwork(containerName, cn)
	default:
		c.Logger.Printf("I do not know how to setup '%s' network", cn.NetworkMode)
	}
}

func (c *Container) setupMacvlanNetwork(containerName string, cn *ContainerNetworkConfig) {
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

func (c *Container) handleContainerNetwork(d *docker.Client) {
	containerInfo, err := containerInfo(d, c.ID)
	if err != nil {
		c.Logger.Errorf("Error inspecting container: %s", err.Error())
	}
	if containerInfo != nil {
		cn := c.getContainerNetworkConfig(containerInfo)

		if cn.NetworkMode != "" {
			c.setupNetwork(containerInfo.Name, &cn)
		}
	}
}
