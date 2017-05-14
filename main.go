package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
)

var dPath string = "/var/run/docker.sock"
var HostLinkName string = "eth0"

func main() {

	f := logrus.TextFormatter{
		DisableColors: false,
		DisableSorting: true,
		DisableTimestamp: false,
		FullTimestamp: true,
		TimestampFormat: "02-01-2006 15:04:05",
	}

	logrus.SetFormatter(&f)

	// Set debug level
	logrus.SetLevel(logrus.InfoLevel)

	//Initialize docker client
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