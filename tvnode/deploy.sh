#!/bin/sh

echo "start go deploy tvnode"

echo "go build for Linux system"
export GOOS=linux
export GOARCH=amd64
go build -a

user=$1
ip=$2

## ssh-copy-id root@192.168.1.103 noneed password
ssh-copy-id "$user@$ip"

echo "terminate tvnode process"
ssh "$user@$ip" 'killall tvnode'

echo "copy tvnode to $user@$ip"
scp ./tvnode ./boot.sh "$user@$ip:/usr/local/bin"
ssh "$user@$ip" 'mv /usr/local/bin/boot.sh /usr/local/bin/tvnode_start.sh'

echo "start tvnode..."
ssh "$user@$ip" '/usr/local/bin/tvnode_start.sh'