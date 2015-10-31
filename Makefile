CMDS=ec2-cluster
REPO=github.com/boynton/hacks
EC2=$(GOPATH)/bin/ec2

all: $(EC2)

clean::
	rm -f *~ $(EC2)

$(EC2): ec2/ec2.go
	go install $(REPO)/ec2
