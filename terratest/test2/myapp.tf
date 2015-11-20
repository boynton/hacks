provider "aws" {
  access_key = "${var.access_key}"
  secret_key = "${var.secret_key}"
  region = "${var.region}"
}

variable "myapp_cidr" {
   default = "10.0.0.0/24"
}

resource "aws_vpc" "myapp" {
  cidr_block = "${var.myapp_cidr}"
  tags {
    Name = "${var.env}.myapp"
    Env = "${var.env}"
  }
}

resource "aws_vpc_peering_connection" "myapp" {
  peer_owner_id = "${var.account_id}"
  peer_vpc_id = "${var.admin_vpc_id}"
  vpc_id = "${aws_vpc.myapp.id}"
  auto_accept = true
  tags {
    Name = "${var.env}.admin:${var.env}.myapp"
    Env = "${var.env}"
  }
}

resource "aws_route" "admin_to_myapp" {
  route_table_id = "${var.admin_route_id}"
  destination_cidr_block = "${var.myapp_cidr}"
  vpc_peering_connection_id = "${aws_vpc_peering_connection.myapp.id}"
}

resource "aws_route" "myapp_to_admin" {
  route_table_id = "${aws_vpc.myapp.main_route_table_id}"
  destination_cidr_block = "${var.admin_cidr}"
  vpc_peering_connection_id = "${aws_vpc_peering_connection.myapp.id}"
}

resource "aws_security_group" "admin" {
  name = "${var.env}.myapp.admin"
  description = "Admin security group for ${var.env}.myapp instances"
  tags {
    Name = "${var.env}.myapp.admin"
    Network = "${var.env}.myapp"
    Env = "${var.env}"
  }
  vpc_id = "${aws_vpc.myapp.id}"
  
  # SSH access from the controlling network
  ingress {
    from_port = 22
    to_port = 22
    protocol = "tcp"
    cidr_blocks = ["${var.admin_cidr}"]
  }
}

resource "aws_subnet" "fe" {
  vpc_id = "${aws_vpc.myapp.id}"
  cidr_block = "10.0.0.0/24"
  tags {
    Name = "${var.env}.myapp.fe"
    Network = "${var.env}.myapp"
    Env = "${var.env}"
  }
}

resource "aws_instance" "webserver" {
  tags {
    Name = "${var.env}.myapp.webserver"
    Network = "${var.env}.myapp"
    Env = "${var.env}"
  }
  instance_type = "t1.micro"
  ami = "${lookup(var.amis, var.region)}"
  key_name = "${var.admin_key_name}"
  subnet_id = "${aws_subnet.fe.id}"
  vpc_security_group_ids = ["${aws_security_group.admin.id}"]
}
