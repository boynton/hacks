# hacks
Little AWS hacks, probably not what you are looking for.

## ec2

A little wrapper to manage a single ec2 instance by name, makes writing shell scripts easier. 

Your AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables need to be set up. You default security profile
needs to allow access to EC2 instances from the machine you run this code on.

```
$ ec2
usage: ec2 [-k] [-n] [-t] [-i] [up,down,id,host,ip,status,ssh,put,get] [other args]
$ ec2 -h
Usage of ec2:
  -i string
      instance image (default "ami-81f7e8b1")
  -k string
      keypair name (default "ec2-user")
  -n string
      instance name (default "default")
  -q  quiet
  -t string
      instance type (default "t1.micro")
  -v  verbose
$ ec2 up
i-9570834c
$ ec2 id
i-9570834c
$ ec2 status
running
$ ec2 host
ec2-54-203-148-35.us-west-2.compute.amazonaws.com
$ ec2 ip
54.203.148.35
$ ec2 ssh hostname
ip-10-249-102-172
$ ec2 ssh ls -l
total 0
$ ec2 put README.md
$ ec2 put README.md r.md
$ ec2 ssh ls -l
total 8
-rw-r--r-- 1 ec2-user ec2-user 61 Oct 30 20:04 README.md
-rw-r--r-- 1 ec2-user ec2-user 61 Oct 30 20:04 r.md
$ ec2 get r.md
$ ls -l
total 24
-rw-r--r--  1 lee  staff  152 Oct 30 13:10 Makefile
-rw-r--r--  1 lee  staff  928 Oct 30 17:22 README.md
drwxr-xr-x  4 lee  staff  136 Oct 30 17:13 ec2
-rw-r--r--  1 lee  staff   61 Oct 30 13:04 r.md
$ ec2 get r.md x.md
$ ls -l
total 32
-rw-r--r--  1 lee  staff  152 Oct 30 13:10 Makefile
-rw-r--r--  1 lee  staff  928 Oct 30 17:22 README.md
drwxr-xr-x  4 lee  staff  136 Oct 30 17:13 ec2
-rw-r--r--  1 lee  staff   61 Oct 30 13:04 r.md
-rw-r--r--  1 lee  staff   61 Oct 30 13:04 x.md
$ ec2 down
```