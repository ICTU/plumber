package main

import (
	"fmt"
	"log"
	"net"

	"github.com/milosgajdos83/tenus"
)

func main() {
	macVlanHost, err := tenus.NewMacVlanLinkWithOptions("eth1", tenus.MacVlanOptions{Mode: "bridge", Dev: "macvlnHost"})
	if err != nil {
		log.Fatal(err)
	}

	macVlanHostIp, macVlanHostIpNet, err := net.ParseCIDR("10.0.41.2/16")
	if err != nil {
		log.Fatal(err)
	}

	if err := macVlanHost.SetLinkIp(macVlanHostIp, macVlanHostIpNet); err != nil {
		fmt.Println(err)
	}

	if err = macVlanHost.SetLinkUp(); err != nil {
		fmt.Println(err)
	}

	macVtapDocker, err := tenus.NewMacVtapLinkWithOptions("eth1", tenus.MacVlanOptions{Mode: "bridge", Dev: "mvtDckrIfc"})
	if err != nil {
		log.Fatal(err)
	}

	pid, err := tenus.DockerPidByName("mvtapdckr", "/var/run/docker.sock")
	if err != nil {
		log.Fatal(err)
	}

	if err := macVtapDocker.SetLinkNetNsPid(pid); err != nil {
		log.Fatal(err)
	}

	macVtapDckrIp, macVtapDckrIpNet, err := net.ParseCIDR("10.0.41.3/16")
	if err != nil {
		log.Fatal(err)
	}

	if err := macVtapDocker.SetLinkNetInNs(pid, macVtapDckrIp, macVtapDckrIpNet, nil); err != nil {
		log.Fatal(err)
	}
}
