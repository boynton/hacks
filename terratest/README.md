# Simple test network

## test1 - Admin

The admin network (VPC) is set up with a bastion subnet containing a jumphost instance.

![admin resources](https://github.com/boynton/hacks/blob/master/terratest/admin.svg)

## test2 - Myapp

This network is a minimal app VPC that peers with the admin network. All access via SSH to the
instance is via the admin network's jumphost.

![myapp resources](https://github.com/boynton/hacks/blob/master/terratest/myapp.svg)
