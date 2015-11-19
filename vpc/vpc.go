package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func fatal(msg string) {
	fmt.Println(msg)
	os.Exit(1)
}

func usage() {
	fatal("usage: vpc [options] [setup,list,describe,create,destroy,create-zone,destroy-zone,run-machine,machines,cleanup] [other args]")
}

var quiet = false
var env = "dev"

func main() {
	defCtrl := os.Getenv("VPC_CTRL")
	if defCtrl == "" {
		defCtrl = "0.0.0.0/0"
	}
	defEnv := os.Getenv("VPC_ENV")
	if defEnv == "" {
		defEnv = "dev"
	}
	pCtrl := flag.String("c", defCtrl, "controlling net block") //the net block of your "home" or controlling machines
	//	pAdmin := flag.String("a", "10.255.255.0/24", "admin net block") //the net block of the 'admin' Network
	pEnv := flag.String("e", defEnv, "environment")
	pQuiet := flag.Bool("q", false, "quiet")
	pImage := flag.String("i", "ami-81f7e8b1", "instance image")
	pType := flag.String("t", "t1.micro", "instance type")
	pKeyname := flag.String("k", "ec2-user", "keypair name")
	flag.Parse()
	args := flag.Args()
	if len(args) > 0 {
		env = *pEnv
		quiet = *pQuiet
		cloud := NamedCloud(env)
		op := args[0]
		switch op {
		case "describe":
			err := cloud.Status()
			if err != nil {
				fatal(err.Error())
			}
			os.Exit(0)
		case "list":
			lst, err := cloud.ListNetworks()
			if err != nil {
				fatal(err.Error())
			}
			fmt.Println(pretty(lst))
			os.Exit(0)
		case "setup":
			err := cloud.Setup(*pCtrl)
			if err != nil {
				fatal(err.Error())
			}
			os.Exit(0)
		case "machines":
			lst, err := cloud.ListMachines()
			if err != nil {
				fatal(err.Error())
			}
			if len(lst) > 0 {
				fmt.Printf("Instances in %s:\n", cloud.Name)
				for _, machine := range lst {
					fmt.Printf("%v\n", machine)
				}
			}
			os.Exit(0)
		case "create":
			if len(args) == 3 {
				name := args[1]
				cidr := args[2]
				_, err := cloud.CreateNetwork(name, cidr)
				if err != nil {
					fatal(err.Error())
				}
				os.Exit(0)
			}
		case "destroy":
			if len(args) == 2 {
				name := args[1]
				err := cloud.DestroyNetwork(name)
				if err != nil {
					fatal(err.Error())
				}
				os.Exit(0)
			}
		case "create-zone":
			if len(args) >= 3 {
				netName := args[1]
				name := args[2]
				net, err := cloud.FindNetwork(netName)
				if err != nil {
					fatal(err.Error())
				}
				if net == nil {
					fatal("No such network: " + netName)
				}
				cidr := net.AddressBlock //default to the entire network
				if len(args) == 3 {
					cidr = args[3]
					//to do: validate that the subnet is in the network
				}
				_, err = net.CreateZone(name, cidr)
				if err != nil {
					fatal(err.Error())
				}
				os.Exit(0)
			}
		case "run-machine":
			if len(args) == 3 {
				name := args[1]
				zoneName := args[2]
				zone, err := cloud.GetZone(zoneName)
				if err != nil {
					fatal(err.Error())
				} else if zone == nil {
					fatal("No such zone: " + zoneName)
				}
				machine, err := cloud.LaunchMachine(zone, name, *pKeyname, *pImage, *pType)
				if err != nil {
					fatal(err.Error())
				}
				fmt.Println("returned", machine)
				os.Exit(0)
			}
		case "ssh":
			if len(args) >= 2 {
				machine, err := cloud.GetMachineById(args[1])
				if err != nil {
					fatal(err.Error())
				}
				if strings.HasSuffix(machine.Name, ".admin.jumphost") {
					result, err := machine.SSH(*pKeyname, args[2:]...)
					if err != nil {
						fatal(err.Error())
					}
					fmt.Print(result)
				} else {
					jumpHost, err := cloud.FindMachine("admin.jumphost")
					if err != nil {
						fatal("Cannot find jumphost to connect through: " + err.Error())
					}
					tmp := []string{"ssh", "-o", "StrictHostKeyChecking=no", machine.PrivateIp()}
					for _, s := range args[2:] {
						tmp = append(tmp, s)
					}
					result, err := jumpHost.SSH(*pKeyname, tmp...)
					if err != nil {
						fatal(err.Error())
					}
					fmt.Print(result)
				}
				os.Exit(0)
			}
		case "cleanup":
			err := cloud.Cleanup()
			if err != nil {
				fatal(err.Error())
			}
			os.Exit(0)
		}
	}
	usage()
}
