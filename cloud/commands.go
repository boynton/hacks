package main

import (
	"fmt"
	"os"
)

func (cloud *Cloud) describeCommand(help bool) {
	if help {
		fmt.Println("describe command help")
		
	} else {
		fmt.Println("describe command")
	}
	os.Exit(0)
}

func (cloud *Cloud) setupCommand(adminCidr string, bastionCidr string, ctrlCidr string) {
	fmt.Println("setup cloud ", adminCidr, bastionCidr, ctrlCidr)
	os.Exit(0)
}

func (cloud *Cloud) cleanupCommand() {
	fmt.Println("cleanup command")
	os.Exit(0)
}

func (cloud *Cloud) listNetworksCommand() {
	fmt.Println("list networks here")
	os.Exit(0)
}

func (cloud *Cloud) describeNetworkCommand(netName string) {
	fmt.Println("describe network '" + netName + "' here")
	os.Exit(0)
}
