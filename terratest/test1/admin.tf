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
  vpc_id = "${aws_vpc.admin.id}"
  
  # SSH access from the controlling network
  ingress {
    from_port = 22
    to_port = 22
    protocol = "tcp"
    cidr_blocks = ["${var.ctrl_network}"]
  }
  egress {
    from_port = 0
    to_port = 0
    protocol = "-1"
    cidr_blocks = ["0.0.0.0/0"]
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
  instance_type = "t1.micro"
  ami = "${lookup(var.amis, var.region)}"
  key_name = "${var.admin_key_name}"
  subnet_id = "${aws_subnet.bastion.id}"
  vpc_security_group_ids = ["${aws_security_group.bastion.id}"]
}

resource "aws_eip" "jumphost_ip" {
  instance = "${aws_instance.jumphost.id}"
  vpc = true

/*
  # This is broken. Some race in getting eip through gateway to running host with sshd running...the "host"
  # in the ssh command ends up blank, so this never succeeds. Note that if the provisioner is in the instance,
  # then it uses the instance's private-ip before the eip is even instantiated! which of course doesn't work.

  # The connection block tells our provisioner how to
  # communicate with the resource (instance)
  connection {
    user = "${var.admin_user}"
    key_file = "${var.admin_key_path}"
  }

  # We run a remote provisioner on the instance after creating it.
  # This is in the Elastic IP, since we need that public IP to
  # talk to it via SSH.
  provisioner "remote-exec" {
    inline = [
      "sudo apt-get -y update",
    ]
  }
*/

}

resource "aws_internet_gateway" "gateway" {
  tags {
    Name = "${var.env}.admin.gateway"
    Network = "${var.env}.admin"
    Env = "${var.env}"
  }
  vpc_id = "${aws_vpc.admin.id}"
}

resource "aws_route" "gateway" {
  route_table_id = "${aws_vpc.admin.main_route_table_id}"
  destination_cidr_block = "0.0.0.0/0"
  gateway_id = "${aws_internet_gateway.gateway.id}"
}
