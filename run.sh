#!/bin/bash
export USER=root
mkdir -p /var/run/sshd
mv /authorized_keys /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys
nohup /usr/sbin/sshd -D &
echo 'PS1='"'"'${debian_chroot:+($debian_chroot)}\[\033[01;32m\]\u\[\033[00m\]:\[\033[01;35;35m\]\w\[\033[00m\]\$\033[1;32;32m\] '"'" >> /root/.bashrc
cd /
chmod +x server
/server 
