package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/urfave/cli"
	"os"
)

var (
	DockerHost   string
	HostLinkName string
	Logger       *logrus.Logger
)

func main() {
	initializeLogger()

	app := initializeApp()

	app.Action = func(c *cli.Context) error {
		DockerHost = c.String("docker-host")
		HostLinkName = c.String("host-link")

		d, err := initializeDocker(DockerHost)
		if err != nil {
			Logger.Fatalf("Failed initializing docker client: %s", err.Error())
		}

		// Add event listener source them to events channel
		events := make(chan *docker.APIEvents)
		d.AddEventListener(events)

		// Process existing containers
		go processExistingContainers(d)

		// Process incoming events
		processIncomingEvents(events, d)

		return nil
	}

	app.Run(os.Args)
}
