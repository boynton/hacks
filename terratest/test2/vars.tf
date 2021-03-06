variable "access_key" {}
variable "secret_key" {}
variable "account_id" {}

variable "region" {
    default = "us-west-2"
}

variable "ctrl_network" {
   description = "The network block to restrict access to the bastion host from"
   default = "0.0.0.0/0"
}

variable "env" {
   default = "terra"
}

variable "admin_user" {
   description = "Username of the remote user to do administration"
   default = "ec2-user"
}

variable "admin_key_name" {
   description = "Name of the SSH keypair to use in AWS."
   default = "ec2-user"
}

variable "key_dir" {
   default = "~/.ssh"
   description = "Path to the local directory where private keys are kept."
}

variable "admin_key_path" {
   default = "~/.ssh/ec2-user.pem"
}

variable "amis" {
    default = {
        us-east-1 = "ami-aa7ab6c2"
        us-west-2 = "ami-81f7e8b1"
    }
}

#these come from an already-existing "admin" vpc. Must provide them at runtime
variable "admin_vpc_id" {}
variable "admin_route_id" {}
variable "admin_cidr" {
   default = "10.255.255.0/24"
}
