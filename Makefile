REPO=github.com/boynton/hacks
EC2=$(GOPATH)/bin/ec2
VPC=$(GOPATH)/bin/vpc
CLOUD=$(GOPATH)/bin/cloud
JSON=$(GOPATH)/bin/json

all: $(CLOUD) $(EC2) $(VPC) $(JSON)

check::
	go fmt $(REPO)/vpc
	go vet $(REPO)/vpc
	go fmt $(REPO)/ec2
	go vet $(REPO)/ec2

clean::
	rm -f *~ $(EC2)

$(EC2): ec2/ec2.go
	go install $(REPO)/ec2

$(VPC): vpc/vpc.go vpc/util.go
	go install $(REPO)/vpc

$(CLOUD): cloud/main.go cloud/cloud.go cloud/commands.go
	go install $(REPO)/cloud

$(JSON): json/main.go
	go install $(REPO)/json
