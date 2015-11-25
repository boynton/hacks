package main

import (
	"fmt"
	"github.com/jawher/mow.cli"
	"os"
)

func main() {
	app := cli.App("cloud", "")
	app.Version("v version", "cloud 0.0.1")
	var cloud *Cloud
	pEnv := app.StringOpt("e env", "dev", "select the environment to use")
	app.Before = func () {
		cloud = NamedCloud(*pEnv)
	}
	app.Command("describe", "", func (cmd *cli.Cmd) {
		cloud.describeCommand(false)
	})
	app.Command("setup", "", func (cmd *cli.Cmd) {
		pAdminNet := cmd.StringOpt("n admin-net-cidr", "10.255.255.0/24", "CIDR of the admin network for the cloud")
		pBastionSubnet := cmd.StringOpt("a admin-bastion-subnet", "10.255.255.192/28", "CIDR of the admin's bastion subnet")
		pControlNet := cmd.StringOpt("c control-net", "0.0.0.0/0", "CIDR of the controlling network to allow SSH from")
		cloud.setupCommand(*pAdminNet, *pBastionSubnet, *pControlNet)
	})
	app.Command("cleanup", "", func (cmd *cli.Cmd) {
		cloud.cleanupCommand()
	})
	app.Command("up", "", func (cmd *cli.Cmd) {
		fmt.Println("bring entire cloud up")
	})
	app.Command("down", "", func (cmd *cli.Cmd) {
		fmt.Println("bring entire cloud down")
	})
	app.Command("net", "", func (cmd *cli.Cmd) {
		pNetName := cmd.StringArg("NAME", "", "the name of the network")
		cmd.Command("describe", "List the network", func (subcmd *cli.Cmd) {
			cloud.describeNetworkCommand(*pNetName)
		})
		cmd.Command("create", "Create the network definition", func (subcmd *cli.Cmd) {
			subcmd.StringOpt("n net-cidr", "10.0.0.0/24", "CIDR of the new network")
			subcmd.StringOpt("a net-cidr", "10.255.255.0/24", "CIDR of the admin network to peer with")
		})
		cmd.Command("destroy", "Destroy the network definition", func (subcmd *cli.Cmd) {
			subcmd.BoolOpt("f --force", false, "force shutdown of all machines in the network")
		})
		cmd.Command("up", "bring up the network", func (subcmd *cli.Cmd) {
		})
		cmd.Command("down", "bring down the network", func (subcmd *cli.Cmd) {
			subcmd.BoolOpt("f --force", false, "force shutdown of all machines in the network")
		})
	})
/*
 --> use terraform to define the machine clusters and how to bring them up. Not this

	app.Command("cluster", "", func (cmd *cli.Cmd) {
		pClusterName := cmd.StringArg("NAME", "", "the name of the machine cluster")
		cmd.Command("describe", "List the network", func (subcmd *cli.Cmd) {
			cloud.describeNetworkCommand(*pNetName)
		})
		cmd.Command("create", "Create the cluster definition", func (subcmd *cli.Cmd) {
			subcmd.StringOpt("t instance-type", "t2.micro", "the type of machine instance to use")
			subcmd.StringOpt("i image", "ami-81f7e8b1", "CIDR of the admin network to peer with")
		})
		cmd.Command("destroy", "Destroy the network definition", func (subcmd *cli.Cmd) {
			subcmd.BoolOpt("f --force", false, "force shutdown of all machines in the network")
		})
		cmd.Command("up", "bring up the network", func (subcmd *cli.Cmd) {
		})
		cmd.Command("down", "bring down the network", func (subcmd *cli.Cmd) {
			subcmd.BoolOpt("f --force", false, "force shutdown of all machines in the network")
		})
		
	})
*/
	//to do: decide how best to expose the "ssh" functionality to a machine instance.
	app.Run(os.Args)
	os.Exit(0)
}
