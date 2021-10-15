#!/bin/bash
export USER=root
mkdir -p /var/run/sshd
mv /authorized_keys /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys
nohup /usr/sbin/sshd -D &
cd /
chmod +x server
/server 
