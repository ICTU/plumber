package main

import (
	"crypto/rand"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
)

func generateMAC() string {
	buf := make([]byte, 6)
	_, err := rand.Read(buf)
	if err != nil {
		fmt.Println("error:", err)
		return ""
	}
	// Set the local bit
	buf[0] = (buf[0] | 2) & 0xfe
	mac := fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5])
	logrus.Debugf("Generated MAC: %v", mac)
	return mac
}

func initializeDocker() (*docker.Client, error) {
	dPath := "unix:///var/run/docker.sock"
	logrus.Printf("Docker client connected to: %s", dPath)
	d, err := docker.NewClient(dPath)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func containerInfo(d *docker.Client, containerID string) (*docker.Container, error) {
	container, err := d.InspectContainer(containerID)
	if err != nil {
		return nil, err
	}
	return container, nil
}
