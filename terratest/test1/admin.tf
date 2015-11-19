provider "aws" {
  access_key = "${var.access_key}"
  secret_key = "${var.secret_key}"
  region = "${var.region}"
}

resource "aws_vpc" "admin" {
  cidr_block = "10.255.255.0/24"
  tags {
    Name = "${var.env}.admin"
    Env = "${var.env}"
  }
}

resource "aws_security_group" "bastion" {
  name = "${var.env}.admin.bastion"
  description = "Bastion security group for ${var.env}.admin.bastion"
  tags {
    Name = "${var.env}.admin.bastion"
    Network = "${var.env}.admin"
    Env = "${var.env}"
  }
  
  # SSH access from the controlling network
  ingress {
    from_port = 22
    to_port = 22
    protocol = "tcp"
    cidr_blocks = ["${var.ctrl_network}"]
  }

}

resource "aws_subnet" "bastion" {
  vpc_id = "${aws_vpc.admin.id}"
  cidr_block = "10.255.255.0/28"
  tags {
    Name = "${var.env}.admin.bastion"
    Network = "${var.env}.admin"
    Env = "${var.env}"
  }
}

resource "aws_instance" "jumphost" {
  tags {
    Name = "${var.env}.admin.jumphost"
    Network = "${var.env}.admin"
    Env = "${var.env}"
  }

  # The connection block tells our provisioner how to
  # communicate with the resource (instance)
  connection {
    user = "${var.admin_user}"
    key_file = "${var.admin_key_path}"
  }

  instance_type = "t1.micro"

  # Lookup the correct AMI based on the region
  # we specified
  ami = "${lookup(var.amis, var.region)}"

  key_name = "${var.admin_key_name}"

  subnet_id = "${aws_subnet.bastion.id}"
#  security_groups = ["${aws_security_group.default.name}"]
  security_groups = ["${aws_security_group.bastion.name}"]

  # We run a remote provisioner on the instance after creating it.
  # In this case, we just install nginx and start it. By default,
  # this should be on port 80
  provisioner "remote-exec" {
    inline = [
      "sudo apt-get -y update",
#      "sudo apt-get -y install nginx",
#      "sudo service nginx start"
    ]
  }
}

resource "aws_internet_gateway" "gateway" {
  tags {
    Name = "${var.env}.admin.gateway"
    Network = "${var.env}.admin"
    Env = "${var.env}"
  }
  vpc_id = "${aws_vpc.admin.id}"
}

resource "aws_eip" "jumphost_ip" {
    instance = "${aws_instance.jumphost.id}"
    vpc = true
}

resource "aws_route_table" "r" {
  vpc_id = "${aws_vpc.admin.id}"
  route {
      cidr_block = "0.0.0.0/0"
      gateway_id = "${aws_internet_gateway.gateway.id}"
  }
  tags {
    Name = "${var.env}.admin.routetable"
    Network = "${var.env}.admin"
    Env = "${var.env}"
  }
}


