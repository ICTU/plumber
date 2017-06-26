package main

import (
	"crypto/rand"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/urfave/cli"
	"net/url"
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
	return mac
}

func initializeApp() *cli.App {
	app := cli.NewApp()
	app.Name = "plumber"
	app.Usage = "network provisioning for docker containers"
	app.Version = version
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "docker-host",
			Value:  "unix:///var/run/docker.sock",
			Usage:  "A tcp or unix connection string",
			EnvVar: "DOCKER_HOST",
		},
		cli.StringFlag{
			Name:  "host-link",
			Value: "eth0",
			Usage: "The name of the host link",
		},
	}
	return app
}

func initializeLogger() {
	Logger = logrus.New()
	Logger.Level = logrus.InfoLevel
	f := logrus.TextFormatter{
		DisableColors:    false,
		DisableSorting:   true,
		DisableTimestamp: false,
		FullTimestamp:    true,
		TimestampFormat:  "02-01-2006 15:04:05",
	}
	Logger.Formatter = &f
}

func initializeDocker(dockerHost string) (*docker.Client, error) {
	Logger.Printf("Docker client connected to: %s", dockerHost)
	d, err := docker.NewClient(dockerHost)
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

// e.g. for unix scheme return /var/run/docker.sock
// e.g. for tcp scheme return 10.19.88.49:2375
func getDockerHostPath(d string) string {
	var dPath string
	myUrl, _ := url.Parse(d)
	if myUrl.IsAbs() {
		switch myUrl.Scheme {
		case "unix":
			dPath = myUrl.Path
			break
		case "tcp":
			dPath = myUrl.Host
			break

		}
	}
	return dPath
}

func processIncomingEvents(events chan *docker.APIEvents, d *docker.Client) {
	Logger.Println("Start listening for docker events")
	for {
		select {
		case event := <-events:
			go func(event *docker.APIEvents) {
				if event.Type == "container" {
					c := NewContainer(event.Actor.ID[0:12])
					c.Name = event.Actor.Attributes["name"]
					switch event.Action {
					case "start":
						c.Logger.Printf("Container '%s' event -> '%s'", c.Name, event.Action)
						c.handleContainerNetwork(d)
					}
				}
			}(event)
		}
	}
}

func processExistingContainers(d *docker.Client) {
	containers, err := d.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		Logger.Fatalf("Failed to get containers: %v", err)
	}

	Logger.Println("Processing existing containers")
	for _, container := range containers {
		go func(container docker.APIContainers) {
			if container.State == "running" {
				c := NewContainer(container.ID[0:12])
				c.handleContainerNetwork(d)
			}
		}(container)
	}
	Logger.Println("All existing containers have been processed")
}
