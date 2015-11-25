package main

import (
	"encoding/json"
	"fmt"
//	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"os"
//	"os/exec"
//	"strings"
//	"time"
)

func pretty(obj interface{}) string {
	b, _ := json.MarshalIndent(obj, "", "   ")
	return string(b)
}

func exit(code int) {
	os.Exit(code)
}

type Cloud struct {
	Name string
	ec2  *ec2.EC2
}

// create a wrapper for the remote named cloud, which may or may not currently exist.
func NamedCloud(name string) *Cloud {
	fmt.Printf("creating cloud for environment '%s'\n", name)
	return &Cloud{Name: name, ec2: ec2.New(session.New())}
}
