package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"os"
)

var DockerSocketPath string = os.Getenv("DOCKER_SOCKET_PATH")
var HostLinkName string = os.Getenv("HOST_LINK_NAME")

func main() {

	initializeLogger()

	d, err := initializeDocker()
	if err != nil {
		logrus.Fatalf("Failed initializing docker client: %s", err.Error())
	}

	// Add event listener source them to events channel
	events := make(chan *docker.APIEvents)
	d.AddEventListener(events)

	// Process incoming events
	logrus.Println("Start listening for docker events")
	for {
		select {
		case event := <-events:
			if event.Type == "container" {
				c := &Container{
					ID: event.Actor.ID[0:12],
				}
				go c.handleContainerEvent(d, event)
			}
		}
	}

}
