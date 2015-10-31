package main

import (
	"flag"
	"fmt"
   "github.com/aws/aws-sdk-go/service/ec2"
   "github.com/aws/aws-sdk-go/aws"
	"os"
	"os/exec"
	"time"
)

func fatal(msg string) {
	fmt.Println(msg)
	os.Exit(1)
}

func usage() {
	fatal("usage: ec2 [-k] [-n] [-t] [-i] [up,down,id,status,wait] [other args]")
}

var verbose = false
var quiet = false

func main() {
	//   ec2 run-instances --image-id ami-81f7e8b1 --count 1 --instance-type t1.micro --key-name docker --security-groups default > .aws-docker-machine
	pName := flag.String("n", "default", "instance name")
	pImage := flag.String("i", "ami-81f7e8b1", "instance image")
	pType := flag.String("t", "t1.micro", "instance type")
	pKeyname := flag.String("k", "ec2-user", "keypair name")
	pVerbose := flag.Bool("v", false, "verbose")
	pQuiet := flag.Bool("q", false, "quiet")
	flag.Parse()
	args := flag.Args()
	if len(args) > 0 {
		verbose = *pVerbose
		quiet = *pQuiet
		op := args[0]
		switch op {
		case "up":
			up(*pName, *pKeyname, *pImage, *pType)
		case "down":
			down(*pName)
		case "id":
			id(*pName)
		case "ip":
			ip(*pName)
		case "host":
			host(*pName)
		case "status":
			status(*pName)
		case "wait":
			wait(*pName, *pKeyname)
		case "ssh":
			ssh(*pName, *pKeyname, args[1:])
		case "put":
			if len(args) > 1 {
				src := args[1]
				dst := src
				if len(args) == 3 {
					dst = args[2]
				}
				putfile(*pName, *pKeyname, src, dst)
			}
		case "get":
			if len(args) > 1 {
				src := args[1]
				dst := src
				if len(args) == 3 {
					dst = args[2]
				}
				getfile(*pName, *pKeyname, src, dst)
			}
		}
	}
	usage()
}

func up(name string, keyname string, instanceImage string, instanceType string) {
	inst, err := findInstance(name)
	if inst != nil && err == nil {
		if verbose {
			fmt.Println("Already running: ", *inst.InstanceId)
		} else {
			fmt.Println(*inst.InstanceId)
		}
		os.Exit(0)
	}
	if verbose {
		fmt.Println("Launching...")
	}
	inst, err = launchInstance(name, keyname, instanceImage, instanceType)
	if inst != nil && err == nil {
		if verbose {
			fmt.Println("Launched", *inst.InstanceId)
		}
		wait(name, keyname)
	}
	os.Exit(1)
}

func down(name string) {
	err := terminateInstance(name)
	if err != nil {
		if verbose {
			fmt.Println("Cannot terminate instance:", err)
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func id(name string) {
	inst, err := findInstance(name)
	if err == nil && inst != nil {
		fmt.Println(*inst.InstanceId)
		os.Exit(0)
	}
	if verbose {
		fmt.Println("Cannot find instance: ", name)
	}
	os.Exit(1)
}

func status(name string) {
	inst, err := findInstance(name)
	if err == nil && inst != nil {
		fmt.Println(*inst.State.Name)
		os.Exit(0)
	}
	os.Exit(1)
}

func ip(name string) {
	inst, err := findInstance(name)
	if err == nil && inst != nil {
		fmt.Println(*inst.PublicIpAddress)
		os.Exit(0)
	}
	if verbose {
		fmt.Println("Cannot find instance: ", name)
	}
	os.Exit(1)
}

func host(name string) {
	inst, err := findInstance(name)
	if err == nil && inst != nil {
		fmt.Println(*inst.PublicDnsName)
		os.Exit(0)
	}
	if verbose {
		fmt.Println("Cannot find instance: ", name)
	}
	os.Exit(1)
}

func wait(name string, keyname string) {
	inst, err := findInstance(name)
	if err == nil && inst != nil {
		instanceId := *inst.InstanceId
		if *inst.State.Name != "running" {
			if *inst.State.Name != "pending" {
				if verbose {
					fmt.Println("cannot wait, instance status is: ", *inst.State.Name)
				}
				os.Exit(1)
			}
			for inst != nil && *inst.State.Name == "pending" {
				delayInSeconds := 3.0
				dur := time.Duration(delayInSeconds * float64(time.Second))
				time.Sleep(dur)
				inst, err = findInstance(name)
				if inst == nil || err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				if verbose {
					fmt.Print(".")
				}
			}
		}
		if *inst.State.Name != "running" {
			if verbose {
				fmt.Println("wait failed, instance status is:", *inst.State.Name)
			}
			os.Exit(1)
		}
		if verbose {
			fmt.Println("Running. Now wait for it to respond to us...")
		}
		output, err := execRemoteCommand(name, keyname)
		for err != nil {
			if verbose {
				fmt.Print(".")
			}
			delayInSeconds := 10.0
			dur := time.Duration(delayInSeconds * float64(time.Second))
			time.Sleep(dur)
			output, err = execRemoteCommand(name, keyname)
			if verbose {
				if err != nil {
					fmt.Println("***", err)
				} else {
					fmt.Print(output)
				}
			}
		}
		if verbose {
			if err != nil {
				fmt.Println("***", err)
			} else {
				fmt.Print(output)
			}
		}
		fmt.Println(instanceId)
		os.Exit(0)
	}
	if verbose {
		fmt.Println("Cannot find instance: ", name)
	}
	os.Exit(1)
}

func putfile(name string, keyname string, src string, dst string) {
	inst, err := findInstance(name)
	if err != nil || inst == nil {
		fmt.Println("Cannot find instance: ", name)
		os.Exit(1)
	}
	host := *inst.PublicIpAddress
	args := make([]string, 0)
	args = append(args, "-rp")
	args = append(args, "-o")
	args = append(args, "StrictHostKeyChecking=no")

	keyfile := os.Getenv("HOME") + "/.ssh/" + keyname + ".pem"
	args = append(args, "-i")
	args = append(args, keyfile)

	args = append(args, src)
	args = append(args, "ec2-user@" + host + ":" + dst)

	if verbose {
		fmt.Print("[scp")
		for _, s := range args {
			fmt.Printf(" %s", s)
		}
		fmt.Println("]")
	}
	out, err := exec.Command("scp", args...).Output()
	if err != nil {
		fmt.Println("Cannot execute scp command:", err)
		os.Exit(1)
	}
	if verbose {
		fmt.Println(string(out))
	}
	os.Exit(0)
}

func getfile(name string, keyname string, src string, dst string) {
	inst, err := findInstance(name)
	if err != nil || inst == nil {
		fmt.Println("Cannot find instance: ", name)
		os.Exit(1)
	}
	host := *inst.PublicIpAddress
	args := make([]string, 0)
	args = append(args, "-rp")
	args = append(args, "-o")
	args = append(args, "StrictHostKeyChecking=no")

	keyfile := os.Getenv("HOME") + "/.ssh/" + keyname + ".pem"
	args = append(args, "-i")
	args = append(args, keyfile)

	args = append(args, "ec2-user@" + host + ":" + src)
	args = append(args, dst)

	if verbose {
		fmt.Print("[scp")
		for _, s := range args {
			fmt.Printf(" %s", s)
		}
		fmt.Println("]")
	}
	out, err := exec.Command("scp", args...).Output()
	if err != nil {
		fmt.Println("Cannot execute scp command:", err)
		os.Exit(1)
	}
	if verbose {
		fmt.Println(string(out))
	}
	os.Exit(0)
}

func execRemoteCommand(name string, keyname string, cmd ...string) (string, error) {
	inst, err := findInstance(name)
	if err != nil {
		return "", err
	}
	if inst == nil {
		return "", fmt.Errorf("Instance not found: %s", name)
	}
	host := *inst.PublicIpAddress
	args := make([]string, 0)
	args = append(args, "-o")
	args = append(args, "StrictHostKeyChecking=no")

	keyfile := os.Getenv("HOME") + "/.ssh/" + keyname + ".pem"
	args = append(args, "-i")
	args = append(args, keyfile)

	args = append(args, "ec2-user@" + host)

	if len(cmd) == 0 {
		args = append(args, "hostname")
	} else {
		for _, s := range cmd {
			args = append(args, s)
		}
	}
	if verbose {
		fmt.Print("[ssh")
		for _, s := range args {
			fmt.Printf(" %s", s)
		}
		fmt.Println("]")
	}
	out, err := exec.Command("ssh", args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func ssh(name string, keyname string, cmd []string) {
	output, err := execRemoteCommand(name, keyname, cmd...)
	if err != nil {
		if !quiet {
			fmt.Println("Cannot ssh: ", err)
		}
		os.Exit(1)
	}
	if verbose || !quiet {
		fmt.Print(output)
	}
	os.Exit(0)
}

// ---

func findInstance(name string) (*ec2.Instance, error) {
	client := ec2.New(nil)
	tname := "tag:Name"
	req := &ec2.DescribeInstancesInput{Filters: []*ec2.Filter{&ec2.Filter{Name: &tname, Values: []*string{&name}}}}
	res, err := client.DescribeInstances(req)
	if err != nil {
		return nil, err
	}
	for _, rez := range res.Reservations {
		for _, inst := range rez.Instances {
			state := *inst.State.Name
			if state == "pending" || state == "running" {
				return inst, nil
			} //treat "stopping", "stopped", "terminating", and "terminated" as nonexistent
		}
	}
	return nil, nil
}

func launchInstance(name string, keyname string, instanceImage string, instanceType string) (*ec2.Instance, error) {
	//launch, tag, and wait for it to be running
	//if already pending, just wait
	client := ec2.New(nil)
	runResult, err := client.RunInstances(&ec2.RunInstancesInput{
		ImageId:      aws.String(instanceImage),
		InstanceType: aws.String(instanceType),
		KeyName:      aws.String(keyname),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(1),
	})
	if err != nil {
		return nil, err
	}
	inst := runResult.Instances[0]
	instanceId := inst.InstanceId
	_, err = client.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{instanceId},
		Tags: []*ec2.Tag{&ec2.Tag{Key: aws.String("Name"), Value: aws.String(name)}},
	})
	if err != nil {
		return nil, err
	}
	return inst, nil
}

func terminateInstance(name string) error {
	inst, err := findInstance(name)
	if err != nil {
		return err
	}
	if inst != nil {
		instanceId := *inst.InstanceId
		client := ec2.New(nil)
		_, err := client.TerminateInstances(&ec2.TerminateInstancesInput{InstanceIds: []*string{aws.String(instanceId)}})
		return err
	}
	return nil
}
