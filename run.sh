#!/bin/bash
export USER=root
mkdir -p /var/run/sshd
nohup /usr/sbin/sshd -D &
cd /
chmod +x server
/server 
