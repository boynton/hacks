package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"os"
	"os/exec"
	"strings"
	"time"
)

func pretty(obj interface{}) string {
	b, _ := json.MarshalIndent(obj, "", "   ")
	return string(b)
}

type Cloud struct {
	Name string
	ec2  *ec2.EC2
}

// a Network represents an AWS VPC (Virtual Private Cloud), containing multiple subnets
type Network struct {
	Cloud        *Cloud `json:"-"`
	Name         string
	Id           string
	AddressBlock string
	vpc          *ec2.Vpc
}

func (cloud *Cloud) newNetwork(vpc *ec2.Vpc) *Network {
	net := &Network{vpc: vpc}
	net.Cloud = cloud
	net.Name = findTag(vpc.Tags, "Name")
	net.Id = *net.vpc.VpcId
	net.AddressBlock = *net.vpc.CidrBlock
	return net
}

func (net *Network) String() string {
	return pretty(net)
}

// a Zone is a subnet/security group in a specific network
type Zone struct {
	Network      *Network
	Name         string
	Id           string
	AddressBlock string
	subnet       *ec2.Subnet
}

func (zone *Zone) String() string {
	return pretty(zone)
}

// a Machine is a convenience wrapper for a (virtual) machine instance
type Machine struct {
	Cloud       *Cloud
	Name        string
	Network     string
	Zone        string
	ec2Instance *ec2.Instance
}

func (machine *Machine) Id() string {
	return *machine.ec2Instance.InstanceId
}

func (machine *Machine) PublicIp() string {
	tmp := machine.ec2Instance.PublicIpAddress
	if tmp == nil {
		return ""
	}
	return *tmp
}

func (machine *Machine) PrivateIp() string {
	tmp := machine.ec2Instance.PrivateIpAddress
	if tmp == nil {
		return ""
	}
	return *tmp
}

func (machine *Machine) String() string {
	s := "{"
	s += "\"id\": \""
	s += *machine.ec2Instance.InstanceId
	s += "\", "

	s += "\"name\": \""
	s += machine.Name
	s += "\", "

	s += "\"net\": \""
	s += machine.Cloud.Name + "." + machine.Network
	s += "\", "
	if machine.ec2Instance.PublicIpAddress != nil {
		s += "\"public-ip\": \""
		s += *machine.ec2Instance.PublicIpAddress
		s += "\", "
	}
	s += "\"private-ip\": \""
	s += *machine.ec2Instance.PrivateIpAddress
	s += "\"}"
	return s
}

// create a wrapper for the remote named cloud, which may or may not currently exist.
func NamedCloud(name string) *Cloud {
	return &Cloud{Name: name, ec2: ec2.New(session.New())}
}

const AdminNetName = "admin"
const AdminNetBlock = "10.255.255.0/24"
const BastionNetBlock = "10.255.255.0/28"

func (cloud *Cloud) createNetwork(name string, cidr string) (*Network, error) {
	fullName := cloud.Name + "." + name
	if !quiet {
		fmt.Printf("Creating network '%s' - %s\n", fullName, cidr)
	}
	vpcOut, err := cloud.ec2.CreateVpc(&ec2.CreateVpcInput{
		CidrBlock:       aws.String(cidr),
		InstanceTenancy: aws.String("default"),
	})
	if err != nil {
		return nil, err
	}
	vpc := vpcOut.Vpc
	vpcId := *vpc.VpcId
	_, err = cloud.ec2.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{aws.String(vpcId)},
		Tags: []*ec2.Tag{
			&ec2.Tag{Key: aws.String("Name"), Value: aws.String(fullName)},
			&ec2.Tag{Key: aws.String("Env"), Value: aws.String(cloud.Name)},
		},
	})
	if err != nil {
		cloud.ec2.DeleteVpc(&ec2.DeleteVpcInput{VpcId: vpc.VpcId})
		return nil, err
	}
	if *vpc.State != "available" {
		if *vpc.State != "pending" {
			if !quiet {
				fmt.Println("cannot wait, VPC status is: ", *vpc.State)
			}
			cloud.ec2.DeleteVpc(&ec2.DeleteVpcInput{VpcId: aws.String(vpcId)})
			return nil, fmt.Errorf("Cannot wait for vpc with state of %v", *vpc.State)
		}
		for vpc != nil && *vpc.State == "pending" {
			delayInSeconds := 2.0
			dur := time.Duration(delayInSeconds * float64(time.Second))
			time.Sleep(dur)
			vpc, err = cloud.findVpc(name)
			if vpc == nil || err != nil {
				cloud.ec2.DeleteVpc(&ec2.DeleteVpcInput{VpcId: aws.String(vpcId)})
				return nil, err
			}
		}
	}
	return cloud.newNetwork(vpc), nil
}

func (cloud *Cloud) Setup(ctrlNetBlock string) error {
	vpc, err := cloud.findVpc(AdminNetName)
	if err != nil {
		return err
	}
	if vpc != nil {
		return fmt.Errorf("Cloud already set up: %s", cloud.Name)
	}
	adminNet, err := cloud.createNetwork(AdminNetName, AdminNetBlock)
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Printf("Created VPC '%s' (%s) - %s", adminNet.Name, *adminNet.vpc.VpcId, *adminNet.vpc.CidrBlock)
	}
	err = cloud.initAdminNetwork(adminNet, ctrlNetBlock)
	if err != nil {
		return err
	}
	return nil
}

func (cloud *Cloud) Status() error {
	vpcAdmin, err := cloud.findVpc(AdminNetName)
	if err == nil && vpcAdmin != nil {
		fmt.Printf("Status of %s:\n", cloud.Name)
		lst, err := cloud.ListNetworks()
		if err != nil {
			return err
		}
		for _, net := range lst {
			fmt.Printf("  network %s (%s) - %s:\n", net.Name, *net.vpc.VpcId, *net.vpc.CidrBlock)
			lstZones, err := net.ListZones()
			if err != nil {
				return err
			}
			for _, zone := range lstZones {
				fmt.Printf("    zone %s (%s) - %s\n", zone.Name, *zone.subnet.SubnetId, *zone.subnet.CidrBlock)
			}
			lst, err := cloud.ListMachines()
			if err != nil {
				return err
			}
			for _, machine := range lst {
				if machine.Network == net.Name {
					pub := machine.PublicIp()
					if pub == "" {
						pub = "(no public ip)"
					}
					fmt.Printf("    machine %s (%s) - %s/%s\n", machine.Name, machine.Id(), machine.PrivateIp(), pub)
				}
			}
		}
		return nil
	}
	return fmt.Errorf("Cloud not set up: %s", cloud.Name)
}

func (net *Network) createSecurityGroup(name string, descr string) (*string, error) {
	vpc := net.vpc
	vpcId := vpc.VpcId
	sgName := net.Name + "." + name
	sg, err := net.Cloud.ec2.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
		Description: aws.String(descr),
		GroupName:   aws.String(sgName),
		VpcId:       vpcId,
	})
	if err != nil {
		return nil, err
	}
	_, err = net.Cloud.ec2.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{sg.GroupId},
		Tags: []*ec2.Tag{
			&ec2.Tag{Key: aws.String("Name"), Value: aws.String(sgName)},
			&ec2.Tag{Key: aws.String("Network"), Value: aws.String(net.Name)},
			&ec2.Tag{Key: aws.String("Env"), Value: aws.String(net.Cloud.Name)},
		},
	})
	return sg.GroupId, nil
}

func (cloud *Cloud) initAdminNetwork(net *Network, ctrlNetBlock string) error {
	sgBastionId, err := net.createSecurityGroup("bastion", "Bastion security group for " + net.Name)
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Println("Created 'bastion' security group for " + cloud.Name + "." + net.Name)
	}

	err = cloud.authorizeInboundAddress(sgBastionId, ctrlNetBlock, "tcp", 22)
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Println("Authorized inbound traffic for tcp/22 from " + ctrlNetBlock + " to the bastion security group")
	}

	if false {
		//to do: figure out the vpc peering, that is what we want to restrict, not this
		err = cloud.revokeOutboundDefault(sgBastionId) //this means it cannot even access AWS itself, or internet anything
		if err != nil {
			return nil
		}
		if !quiet {
			fmt.Println("Revoked default outbound rule from bastion security group")
		}
	}

	bastionZone, err := net.CreateZone("bastion", BastionNetBlock)
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Printf("Created zone: %v\n", bastionZone)
	}

	gatewayName := net.Name + ".gateway"
	gw, err := cloud.ec2.CreateInternetGateway(&ec2.CreateInternetGatewayInput{})
	if err != nil {
		return err
	}
	gwId := gw.InternetGateway.InternetGatewayId
	_, err = cloud.ec2.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{gw.InternetGateway.InternetGatewayId},
		Tags: []*ec2.Tag{
			&ec2.Tag{Key: aws.String("Name"), Value: aws.String(gatewayName)},
			&ec2.Tag{Key: aws.String("Network"), Value: aws.String(net.Name)},
			&ec2.Tag{Key: aws.String("Env"), Value: aws.String(cloud.Name)},
		},
	})
	if !quiet {
		fmt.Printf("Created internet gateway '%s' (%s)\n", gatewayName, *gwId)
	}
	_, err = cloud.ec2.AttachInternetGateway(&ec2.AttachInternetGatewayInput{
		VpcId:             net.vpc.VpcId,
		InternetGatewayId: gwId,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Internet gateway '%s' attached to admin.bastion\n", gatewayName)

	if !quiet {
		fmt.Println("Launching jumphost...")
	}
	//launch the jumphost
	keyName := "ec2-user"
	instanceImage := "ami-81f7e8b1"
	instanceType := "t1.micro"
	instance, err := cloud.launchInstance(bastionZone, "jumphost", keyName, sgBastionId, instanceImage, instanceType)
	if err != nil {
		return err
	}
	instanceId := *instance.InstanceId
	fmt.Printf("\nJumphost launched: %s\n", instanceId)

	//set up an EIP
	eip, err := cloud.ec2.AllocateAddress(&ec2.AllocateAddressInput{Domain: aws.String(ec2.DomainTypeVpc)})
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Println("Allocated new elastic IP: ", *eip.PublicIp)
	}

	_, err = cloud.ec2.AssociateAddress(&ec2.AssociateAddressInput{
		AllocationId:       eip.AllocationId,
		AllowReassociation: aws.Bool(true),
		InstanceId:         instance.InstanceId,
	})
	if err != nil {
		return err
	}
	if !quiet {
		i, _ := cloud.getInstance(instanceId)
		fmt.Println("Associated Elastic IP with the newly launched jumphost instance: ", *i.PublicIpAddress)
	}

	rt, err := cloud.ec2.DescribeRouteTables(&ec2.DescribeRouteTablesInput{Filters: []*ec2.Filter{filter("vpc-id", *net.vpc.VpcId)}})
	if err != nil {
		return err
	}
	routeTableId := rt.RouteTables[0].RouteTableId
	_, err = cloud.ec2.CreateRoute(&ec2.CreateRouteInput{
		RouteTableId:         routeTableId,
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            gwId,
	})
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Println("Added default route to internet gateway")
	}
	err = cloud.waitForInstance(instance, keyName)
	if err == nil && !quiet {
		fmt.Println("Admin network is active")
	}
	return err
}

func (cloud *Cloud) findVpc(name string) (*ec2.Vpc, error) {
	fullName := cloud.Name + "." + name
	req := &ec2.DescribeVpcsInput{}
	if name != "" {
		req.Filters = []*ec2.Filter{filter("tag:Name", fullName)}
	}
	res, err := cloud.ec2.DescribeVpcs(req)
	if err != nil {
		return nil, err
	}
	//should only be 1 of them
	for _, vpc := range res.Vpcs {
		return vpc, nil
	}
	return nil, nil
}

func (cloud *Cloud) FindNetwork(name string) (*Network, error) {
	vpc, err := cloud.findVpc(name)
	if err != nil {
		return nil, err
	}
	if vpc == nil {
		return nil, nil
	}
	return cloud.newNetwork(vpc), nil
}

func findTag(tags []*ec2.Tag, key string) string {
	for _, tag := range tags {
		if *tag.Key == key {
			return *tag.Value
		}
	}
	return ""
}

func (cloud *Cloud) ListNetworks() ([]*Network, error) {
	res, err := cloud.ec2.DescribeVpcs(&ec2.DescribeVpcsInput{Filters: []*ec2.Filter{filter("tag:Env", cloud.Name)}})
	if err != nil {
		return nil, err
	}
	if len(res.Vpcs) == 0 {
		return nil, fmt.Errorf("run 'vpc setup' before using")
	}
	lst := make([]*Network, 0)
	for _, vpc := range res.Vpcs {
		lst = append(lst, cloud.newNetwork(vpc))
	}
	return lst, nil
}

func (cloud *Cloud) CreateNetwork(vpcName string, cidr string) (*Network, error) {
	vpc, err := cloud.findVpc(vpcName)
	if err != nil {
		return nil, err
	}
	if vpc != nil {
		return nil, fmt.Errorf("Network already exists in %s: %s", cloud.Name, vpcName)
	}
	net, err := cloud.createNetwork(vpcName, cidr)
	if err != nil {
		return nil, err
	}
	err = cloud.initAppNetwork(net)
	if err != nil {
		return nil, err
	}
	return net, nil
}

func (cloud *Cloud) DestroyNetwork(vpcName string) error {
	vpc, err := cloud.findVpc(vpcName)
	if err != nil {
		return err
	}
	if vpc != nil {
		net := cloud.newNetwork(vpc)

		//delete all peering connections involving this vpc. For now, rely on tags. Should make this more robust re: hand edits
		lstPeers, err := cloud.ec2.DescribeVpcPeeringConnections(&ec2.DescribeVpcPeeringConnectionsInput{
			Filters: []*ec2.Filter{filter("tag:Env", cloud.Name)},
		})
		if err == nil {
			for _, peering := range lstPeers.VpcPeeringConnections {
				for _, tag := range peering.Tags {
					p1_p2 := strings.Split(*tag.Key, ":")
					if len(p1_p2) == 2 {
						fmt.Println("delete peering tag: ", p1_p2)
						if p1_p2[0] == vpcName || p1_p2[1] == vpcName {
							_, err := cloud.ec2.DeleteVpcPeeringConnection(&ec2.DeleteVpcPeeringConnectionInput{VpcPeeringConnectionId: peering.VpcPeeringConnectionId})
							name := findTag(peering.Tags, "Name")
							if err != nil {
								fmt.Printf("Failed to delete VPC peering (%s): %s\n", name, err.Error())
							} else if !quiet {
								fmt.Printf("Deleted VPC peering connection (%s)\n", name)
							}
						}
					}
				}
			}
		}
		//bring down all running instances. And wait for them to terminate (takes a while)
		net.killAllInstances()
		//and destroy the vpc, releasing all its resources
		err = cloud.destroyVpc(vpc, cloud.Name)
		if err != nil {
			fmt.Printf("Failed to destroy network '%s': %v\n", vpcName, err)
		}
	}
	return nil
}

func (cloud *Cloud) GetZone(zoneName string) (*Zone, error) {
	lst := strings.Split(zoneName, ".")
	if len(lst) != 3 || lst[0] != cloud.Name {
		return nil, fmt.Errorf("Zone name must be fully specified, i.e. " + cloud.Name + ".{network}.{zone}")
	}
	net, err := cloud.FindNetwork(lst[1])
	if err != nil {
		return nil, err
	}
	if net == nil {
		return nil, fmt.Errorf("No such network: %s.%s", lst[0], lst[1])
	}
	req := &ec2.DescribeSubnetsInput{Filters: []*ec2.Filter{filter("tag:Name", zoneName)}}
	res, err := cloud.ec2.DescribeSubnets(req)
	if err != nil {
		return nil, err
	}
	fmt.Println("GetSubnets -> ", pretty(res))
	if len(res.Subnets) == 1 {
		return net.newZone(res.Subnets[0]), nil
	}
	return nil, nil
}

func (cloud *Cloud) initAppNetwork(net *Network) error {
	vpc := net.vpc

	adminVpc, err := cloud.findVpc(AdminNetName)
	if err != nil {
		return err
	}
	//this p2p relationship is requested, then accepted. Who requests, who approves?
	//both are in my account, so it doesn't matter. But it will: If the admin account is protected, then you must
	//ask its permission to join the management group.
	//so: a dev or se creates a new VPC, then wants it to be managed, so sends this
	//the "peer" is what you set up a route to. So, the originator must be the adminNetwork, the peer the new network
	peerOut, err := cloud.ec2.CreateVpcPeeringConnection(&ec2.CreateVpcPeeringConnectionInput{PeerVpcId: vpc.VpcId, VpcId: adminVpc.VpcId})
	peeringId := peerOut.VpcPeeringConnection.VpcPeeringConnectionId
	//so, this accept should be done by the admin side.
	//if peer.account == this account, then {
	_, err = cloud.ec2.AcceptVpcPeeringConnection(&ec2.AcceptVpcPeeringConnectionInput{VpcPeeringConnectionId: peeringId})
	if err != nil {
		return err
	}
	peeringName := cloud.Name + "." + AdminNetName + ":" + net.Name
	_, err = cloud.ec2.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{peeringId},
		Tags: []*ec2.Tag{
			&ec2.Tag{Key: aws.String("Name"), Value: aws.String(peeringName)},
			&ec2.Tag{Key: aws.String("Env"), Value: aws.String(cloud.Name)},
		},
	})
	if err != nil {
		return err
	}
	rt, err := cloud.ec2.DescribeRouteTables(&ec2.DescribeRouteTablesInput{Filters: []*ec2.Filter{filter("vpc-id", *adminVpc.VpcId)}})
	if err != nil {
		return err
	}
	routeTableId := rt.RouteTables[0].RouteTableId
	_, err = cloud.ec2.CreateRoute(&ec2.CreateRouteInput{
		RouteTableId:           routeTableId,
		DestinationCidrBlock:   vpc.CidrBlock, //the entire app vpc
		VpcPeeringConnectionId: peeringId,
	})
	if err != nil {
		return err
	}

	rt2, err := cloud.ec2.DescribeRouteTables(&ec2.DescribeRouteTablesInput{Filters: []*ec2.Filter{filter("vpc-id", *vpc.VpcId)}})
	if err != nil {
		return err
	}
	routeTableId = rt2.RouteTables[0].RouteTableId
	_, err = cloud.ec2.CreateRoute(&ec2.CreateRouteInput{
		RouteTableId:           routeTableId,
		DestinationCidrBlock:   adminVpc.CidrBlock, //the entire admin block. Hmm.
		VpcPeeringConnectionId: peeringId,
	})
	if err != nil {
		return err
	}

	tmp, err := cloud.ec2.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{Filters: []*ec2.Filter{filter("vpc-id", *vpc.VpcId)}})
	if err != nil {
		return err
	}
	err = cloud.authorizeInboundAddress(tmp.SecurityGroups[0].GroupId, *adminVpc.CidrBlock, "tcp", 22)
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Println("Authorized inbound traffic for tcp/22 from " + *adminVpc.CidrBlock + " to " + net.Name)
	}

	if !quiet {
		fmt.Println("Set up peering from", *adminVpc.VpcId, "to", *vpc.VpcId, "and route for", *vpc.CidrBlock)
	}
	return err
}

func (net *Network) newZone(subnet *ec2.Subnet) *Zone {
	result := &Zone{Network: net, Name: findTag(subnet.Tags, "Name"), subnet: subnet}
	result.Id = *subnet.SubnetId
	result.AddressBlock = *subnet.CidrBlock
	return result
}

func (net *Network) CreateZone(subnetName string, cidr string) (*Zone, error) {
	subnet, err := net.createSubnet(subnetName, cidr)
	if err != nil {
		return nil, err
	}
	return net.newZone(subnet), nil
}

func (cloud *Cloud) LaunchMachine(zone *Zone, tagName string, keyName string, instanceImage string, instanceType string) (*Machine, error) {
	var sgName *string
	//the default sg needs to allow tcp/22 from 10.255.255.0/24 !!!
	if sgName == nil {
		//grab from the Network itself
		s, err := cloud.FindSecurityGroup(zone.Network.Name + ".default-sg")
		if err == nil {
			sgName = &s
			fmt.Println("using default security group: ", s)
		}
	}
	instance, err := cloud.launchInstance(zone, tagName, keyName, sgName, instanceImage, instanceType)
	if err != nil {
		return nil, err
	}
	return cloud.newMachine(instance), nil
}

func (cloud *Cloud) authorizeInboundAddress(secId *string, addr string, protocol string, port int) error {
	_, err := cloud.ec2.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    secId,
		CidrIp:     aws.String(addr),
		FromPort:   aws.Int64(int64(port)),
		ToPort:     aws.Int64(int64(port)),
		IpProtocol: aws.String(protocol),
	})
	return err
}

func (cloud *Cloud) authorizeInboundGroup(secId *string, group *string, owner *string, protocol string, port int) error {
	_, err := cloud.ec2.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: secId,
		IpPermissions: []*ec2.IpPermission{&ec2.IpPermission{
			FromPort:         aws.Int64(int64(port)),
			IpProtocol:       aws.String(protocol),
			ToPort:           aws.Int64(int64(port)),
			UserIdGroupPairs: []*ec2.UserIdGroupPair{&ec2.UserIdGroupPair{GroupId: group, UserId: owner}},
		}},
	})
	return err
}

func (cloud *Cloud) authorizeOutboundAddress(secId *string, addr string, protocol string, port int) error {
	_, err := cloud.ec2.AuthorizeSecurityGroupEgress(&ec2.AuthorizeSecurityGroupEgressInput{
		GroupId: secId,
		IpPermissions: []*ec2.IpPermission{&ec2.IpPermission{
			FromPort:   aws.Int64(int64(port)),
			IpProtocol: aws.String(protocol),
			ToPort:     aws.Int64(int64(port)),
			IpRanges:   []*ec2.IpRange{&ec2.IpRange{CidrIp: aws.String(addr)}},
		}},
	})
	return err
}

func (cloud *Cloud) authorizeOutboundGroup(secId *string, group *string, owner *string, protocol string, port int) error {
	_, err := cloud.ec2.AuthorizeSecurityGroupEgress(&ec2.AuthorizeSecurityGroupEgressInput{
		GroupId: secId,
		IpPermissions: []*ec2.IpPermission{&ec2.IpPermission{
			FromPort:         aws.Int64(int64(port)),
			IpProtocol:       aws.String(protocol),
			ToPort:           aws.Int64(int64(port)),
			UserIdGroupPairs: []*ec2.UserIdGroupPair{&ec2.UserIdGroupPair{GroupId: group, UserId: owner}},
		}},
	})
	return err
}

func (cloud *Cloud) revokeOutboundDefault(secId *string) error {
	//remove the default security egress rule
	_, err := cloud.ec2.RevokeSecurityGroupEgress(&ec2.RevokeSecurityGroupEgressInput{
		GroupId: secId,
		IpPermissions: []*ec2.IpPermission{&ec2.IpPermission{
			FromPort:   aws.Int64(-1),
			IpProtocol: aws.String("-1"),
			ToPort:     aws.Int64(-1),
			IpRanges:   []*ec2.IpRange{&ec2.IpRange{CidrIp: aws.String("0.0.0.0/0")}},
		}},
	})
	return err
}

func (net *Network) ListZones() ([]*Zone, error) {
	cloud := net.Cloud
	out, err := cloud.ec2.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{filter("tag:Network", net.Name)},
	})
	if err != nil {
		return nil, err
	}
	lst := make([]*Zone, 0)
	for _, sn := range out.Subnets {
		lst = append(lst, net.newZone(sn))
	}
	return lst, nil
}

func (net *Network) createSubnet(name string, cidr string) (*ec2.Subnet, error) {
	subnetName := net.Name + "." + name
	cloud := net.Cloud
	subnet, err := cloud.ec2.CreateSubnet(&ec2.CreateSubnetInput{
		VpcId:     net.vpc.VpcId,
		CidrBlock: aws.String(cidr),
	})
	if err != nil {
		return nil, err
	}
	_, err = cloud.ec2.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{subnet.Subnet.SubnetId},
		Tags: []*ec2.Tag{
			&ec2.Tag{Key: aws.String("Name"), Value: aws.String(subnetName)},
			&ec2.Tag{Key: aws.String("Network"), Value: aws.String(net.Name)},
			&ec2.Tag{Key: aws.String("Env"), Value: aws.String(cloud.Name)},
		},
	})
	if err != nil {
		return nil, err
	}
	return subnet.Subnet, nil
}

func filter(key string, value string) *ec2.Filter {
	return &ec2.Filter{Name: aws.String(key), Values: []*string{aws.String(value)}}
}

func (cloud *Cloud) destroyVpc(vpc *ec2.Vpc, name string) error {
	//to do: terminate all instances, or abort if any exist, or something
	vpcFilter := filter("vpc-id", *vpc.VpcId)
	subnetRes, err := cloud.ec2.DescribeSubnets(&ec2.DescribeSubnetsInput{Filters: []*ec2.Filter{vpcFilter}})
	if err == nil {
		for _, subnet := range subnetRes.Subnets {
			id := *subnet.SubnetId
			_, err := cloud.ec2.DeleteSubnet(&ec2.DeleteSubnetInput{SubnetId: subnet.SubnetId})
			if err != nil {
				fmt.Printf("Cannot delete subnet '%s': %s\n", id, err.Error())
			} else if !quiet {
				fmt.Printf("Deleted subnet '%s'\n", id)
			}
		}
	}
	tmp, err := cloud.ec2.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{Filters: []*ec2.Filter{vpcFilter}})
	if err == nil {
		for _, grp := range tmp.SecurityGroups {
			s := *grp.GroupName
			if s != "default" {
				id := *grp.GroupId
				_, err = cloud.ec2.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{GroupId: grp.GroupId})
				if err != nil {
					fmt.Printf("Cannot delete security group '%s': %s\n", *grp.GroupName, err.Error())
				} else if !quiet {
					fmt.Printf("Deleted security group '%s'\n", id)
				}
			}
		}
	}
	gws, err := cloud.ec2.DescribeInternetGateways(&ec2.DescribeInternetGatewaysInput{Filters: []*ec2.Filter{vpcFilter}})
	//	gws, err := cloud.ec2.DescribeInternetGateways(&ec2.DescribeInternetGatewaysInput{})
	if err == nil {
		for _, gw := range gws.InternetGateways {
			id := *gw.InternetGatewayId
			_, err := cloud.ec2.DetachInternetGateway(&ec2.DetachInternetGatewayInput{
				VpcId:             vpc.VpcId,
				InternetGatewayId: gw.InternetGatewayId,
			})
			if err != nil {
				fmt.Printf("Cannot detach internet gateway '%s': %s\n", findTag(gw.Tags, "Name"), err.Error())
			} else {
				fmt.Printf("Detached internet gateway '%s' from %s\n", findTag(gw.Tags, "Name"), *vpc.VpcId)
			}
			_, err = cloud.ec2.DeleteInternetGateway(&ec2.DeleteInternetGatewayInput{InternetGatewayId: gw.InternetGatewayId})
			if err != nil {
				fmt.Printf("Cannot delete internet gateway '%s': %s\n", findTag(gw.Tags, "Name"), err.Error())
			} else if !quiet {
				fmt.Printf("Deleted internet gateway '%s'\n", id)
			}
		}
	}

	_, err = cloud.ec2.DeleteVpc(&ec2.DeleteVpcInput{VpcId: vpc.VpcId})
	return err
}

func (net *Network) listInstances() ([]*ec2.Instance, error) {
	req := &ec2.DescribeInstancesInput{Filters: []*ec2.Filter{filter("tag:Network", net.Name)}}
	res, err := net.Cloud.ec2.DescribeInstances(req)
	if err != nil {
		return nil, err
	}
	lst := make([]*ec2.Instance, 0)
	for _, rez := range res.Reservations {
		for _, inst := range rez.Instances {
			state := *inst.State.Name
			if state == "pending" || state == "running" {
				lst = append(lst, inst)
			} //treat "stopping", "stopped", "terminating", and "terminated" as nonexistent
		}
	}
	return lst, nil
}

func (net *Network) killAllInstances() error {
	lst, err := net.listInstances()
	if err != nil {
		return err
	}
	for _, inst := range lst {
		id := *inst.VpcId
		fmt.Println("killing instance", id, "...")
		err := net.Cloud.terminateInstance(inst)
		if true {
			i, err := net.Cloud.getInstance(id)
			fmt.Println("...", err, pretty(i))
		}
		if err != nil {
			return err
		}
		if !quiet {
			fmt.Printf("\nDeleted network '%s'\n", id)
		}
	}
	return nil
}

func (cloud *Cloud) Cleanup() error {
	//destroy any peering between vpcs
	lstPeers, err := cloud.ec2.DescribeVpcPeeringConnections(&ec2.DescribeVpcPeeringConnectionsInput{
		Filters: []*ec2.Filter{filter("tag:Env", cloud.Name)},
	})
	if err == nil {
		for _, peering := range lstPeers.VpcPeeringConnections {
			_, err := cloud.ec2.DeleteVpcPeeringConnection(&ec2.DeleteVpcPeeringConnectionInput{VpcPeeringConnectionId: peering.VpcPeeringConnectionId})
			name := findTag(peering.Tags, "Name")
			if err != nil {
				fmt.Printf("Failed to delete VPC peering (%s): %s\n", name, err.Error())
			} else if !quiet {
				fmt.Printf("Deleted VPC peering connection (%s)\n", name)
			}
		}
	}
	lst, err := cloud.ListNetworks()
	if err != nil {
		return err
	}
	if lst != nil && len(lst) > 0 {
		for _, net := range lst {
			net.killAllInstances()
			err := cloud.destroyVpc(net.vpc, cloud.Name)
			if err != nil {
				fmt.Printf("Failed to destroy network: %v\n", err)
			}
		}
	}
	//release any EIPs that are no longer associated with anything
	eips, err := cloud.ec2.DescribeAddresses(&ec2.DescribeAddressesInput{})
	if err != nil {
		return err
	}
	for _, addr := range eips.Addresses {
		if addr.PrivateIpAddress == nil {
			ip := *addr.PublicIp
			_, err = cloud.ec2.ReleaseAddress(&ec2.ReleaseAddressInput{AllocationId: addr.AllocationId})
			if err != nil {
				fmt.Println("warning: failed to release EIP: ", pretty(addr))
			} else if !quiet {
				fmt.Println("Released Address ", ip)
			}
		}
	}
	return nil
}

func (cloud *Cloud) newMachine(ec2Instance *ec2.Instance) *Machine {
	inst := &Machine{Cloud: cloud, ec2Instance: ec2Instance}
	for _, tag := range ec2Instance.Tags {
		if *tag.Key == "Name" {
			inst.Name = *tag.Value
		} else if *tag.Key == "Network" {
			inst.Network = *tag.Value
		}
	}
	return inst
}

func (cloud *Cloud) FindSecurityGroup(name string) (string, error) {
	tmp, err := cloud.ec2.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{Filters: []*ec2.Filter{filter("tag:Name", name)}})
	if err != nil {
		return "", err
	}
	if len(tmp.SecurityGroups) != 1 {
		return "", fmt.Errorf("Security group not found: " + name)
	}
	return *tmp.SecurityGroups[0].GroupId, nil
}

//instance names are in a single global namespace for the network, not scoped by subnet.
func (cloud *Cloud) FindMachine(name string) (*Machine, error) {
	instName := cloud.Name + "." + name
	req := &ec2.DescribeInstancesInput{Filters: []*ec2.Filter{filter("tag:Name", instName)}}
	res, err := cloud.ec2.DescribeInstances(req)
	if err != nil {
		return nil, err
	}
	for _, rez := range res.Reservations {
		for _, inst := range rez.Instances {
			state := *inst.State.Name
			if state == "pending" || state == "running" {
				return cloud.newMachine(inst), nil
			} //treat "stopping", "stopped", "terminating", and "terminated" as nonexistent
		}
	}
	return nil, nil
}

//fix: only one interface in this API.
func (cloud *Cloud) launchInstance(zone *Zone, name string, keyname string, securityGroupId *string, instanceImage string, instanceType string) (*ec2.Instance, error) {
	netName := zone.Network.Name
	instName := netName + "." + name
	net := zone.Network

	//launch, tag, and wait for it to be running
	//if already pending, just wait
	runResult, err := cloud.ec2.RunInstances(&ec2.RunInstancesInput{
		SubnetId:         zone.subnet.SubnetId,
		SecurityGroupIds: []*string{securityGroupId},
		ImageId:          aws.String(instanceImage),
		InstanceType:     aws.String(instanceType),
		KeyName:          aws.String(keyname),
		MinCount:         aws.Int64(1),
		MaxCount:         aws.Int64(1),
	})
	if err != nil {
		return nil, err
	}
	inst := runResult.Instances[0]
	instanceId := inst.InstanceId
	_, err = cloud.ec2.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{instanceId},
		Tags: []*ec2.Tag{
			&ec2.Tag{Key: aws.String("Name"), Value: aws.String(instName)},
			&ec2.Tag{Key: aws.String("Network"), Value: aws.String(net.Name)},
			&ec2.Tag{Key: aws.String("Env"), Value: aws.String(cloud.Name)},
		},
	})
	if err != nil {
		return nil, err
	}
	err = cloud.waitForInstanceState(inst, "running")
	if err != nil {
		return nil, err
	}
	return inst, nil
}

func (cloud *Cloud) waitForInstanceState(inst *ec2.Instance, finalState string) error {
	transitionState := "pending"
	if finalState == "terminated" {
		transitionState = "shutting-down"
	}
	instId := *inst.InstanceId
	inst, err := cloud.getInstance(instId)
	if err == nil && inst != nil {
		if *inst.State.Name != finalState {
			if *inst.State.Name != transitionState {
				return fmt.Errorf("Cannot wait, instance status is: %s", *inst.State.Name)
			}
			for inst != nil && *inst.State.Name == transitionState {
				delayInSeconds := 3.0
				dur := time.Duration(delayInSeconds * float64(time.Second))
				time.Sleep(dur)
				inst, err = cloud.getInstance(instId)
				if err != nil {
					return err
				}
				if inst == nil {
					return fmt.Errorf("Cannot wait: instance '%s' disappeared", instId)
				}
				if !quiet {
					fmt.Print(".")
				}
			}
		}
	}
	return nil
}

func (cloud *Cloud) waitForInstance(inst *ec2.Instance, keyname string) error {
	instId := *inst.InstanceId
	for {
		delayInSeconds := 5.0
		dur := time.Duration(delayInSeconds * float64(time.Second))
		time.Sleep(dur)
		inst, err := cloud.getInstance(instId)
		if err != nil {
			return err
		}
		_, err = cloud.execRemoteCommand(inst, keyname)
		if err == nil {
			return nil
		}
	}
}

//func (cloud *Cloud) LaunchMachine(zone *Zone, name string, keyname string, instanceImage string, instanceType string) (*ec2.Instance, error) {

func (cloud *Cloud) terminateInstance(inst *ec2.Instance) error {
	_, err := cloud.ec2.TerminateInstances(&ec2.TerminateInstancesInput{InstanceIds: []*string{inst.InstanceId}})
	if err != nil {
		return err
	}
	err = cloud.waitForInstanceState(inst, "terminated")
	return err
}

func (machine *Machine) SSH(keyname string, remoteCommand ...string) (string, error) {
	return machine.Cloud.execRemoteCommand(machine.ec2Instance, keyname, remoteCommand...)
}

func (cloud *Cloud) execRemoteCommand(inst *ec2.Instance, keyname string, remoteCommand ...string) (string, error) {
	if inst.PublicIpAddress == nil {
		return "", fmt.Errorf("No public address on target host")
	}
	host := *inst.PublicIpAddress
	cmd := "ssh"
	args := make([]string, 0)
	keyfile := os.Getenv("HOME") + "/.ssh/" + keyname + ".pem"
	args = append(args, "-t")
	args = append(args, "-A")
	args = append(args, "-q")
	args = append(args, "-o")
	args = append(args, "StrictHostKeyChecking=no")

	args = append(args, "-i")
	args = append(args, keyfile)

	args = append(args, "ec2-user@"+host)

	if len(remoteCommand) == 0 {
		args = append(args, "hostname")
	} else {
		for _, s := range remoteCommand {
			args = append(args, s)
		}
	}
	if !quiet {
		fmt.Print("[" + cmd)
		for _, s := range args {
			fmt.Printf(" %s", s)
		}
		fmt.Println("]")
	}
	out, err := exec.Command(cmd, args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (cloud *Cloud) GetMachineById(instId string) (*Machine, error) {
	inst, err := cloud.getInstance(instId)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("Machine not found with id %s", instId)
	}
	return cloud.newMachine(inst), nil
}

func (cloud *Cloud) getInstance(instId string) (*ec2.Instance, error) {
	req := &ec2.DescribeInstancesInput{Filters: []*ec2.Filter{filter("instance-id", instId)}}
	res, err := cloud.ec2.DescribeInstances(req)
	if err != nil {
		return nil, err
	}
	for _, rez := range res.Reservations {
		for _, inst := range rez.Instances {
			return inst, nil
		}
	}
	return nil, nil
}

func (cloud *Cloud) listInstances() ([]*ec2.Instance, error) {
	req := &ec2.DescribeInstancesInput{Filters: []*ec2.Filter{filter("tag:Env", cloud.Name)}}
	res, err := cloud.ec2.DescribeInstances(req)
	if err != nil {
		return nil, err
	}
	lst := make([]*ec2.Instance, 0)
	for _, rez := range res.Reservations {
		for _, inst := range rez.Instances {
			state := *inst.State.Name
			if state == "pending" || state == "running" {
				lst = append(lst, inst)
			} //treat "stopping", "stopped", "terminating", and "terminated" as nonexistent
		}
	}
	return lst, nil
}

func (cloud *Cloud) ListMachines() ([]*Machine, error) {
	lst, err := cloud.listInstances()
	if err != nil {
		return nil, err
	}
	machines := make([]*Machine, 0, len(lst))
	for _, inst := range lst {
		machines = append(machines, cloud.newMachine(inst))
	}
	return machines, nil
}
