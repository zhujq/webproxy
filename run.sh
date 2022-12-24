#!/bin/bash
export USER=root
mkdir -p /var/run/sshd
# mv /authorized_keys /root/.ssh/authorized_keys
# mv /id_rsa /root/.ssh/id_rsa
mv /id_rsa.pub /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys
#chmod 600 /root/.ssh/id_rsa
#chmod 644 /root/.ssh/id_rsa.pub
nohup /usr/sbin/sshd -D &
echo 'PS1='"'"'${debian_chroot:+($debian_chroot)}\[\033[01;32m\]\u\[\033[00m\]:\[\033[01;35;35m\]\w\[\033[00m\]\$\033[1;32;32m\] '"'" >> /root/.bashrc

mkdir -p /root/tail
cd /root/tail
wget https://pkgs.tailscale.com/stable/${TSFILE}
tar xzf ${TSFILE} --strip-components=1
mkdir -p /var/run/tailscale /var/cache/tailscale /var/lib/tailscale
nohup ./tailscaled --tun=userspace-networking --socks5-server=localhost:1055 &
./tailscale up --authkey=${TAILSCALE_AUTHKEY} --hostname=railway-vps

mkdir -p /v2ray
cd /v2ray
wget -O v2ray.zip http://github.com/v2fly/v2ray-core/releases/latest/download/v2ray-linux-64.zip
unzip v2ray.zip 
if [ ! -f "v2ray" ]; then
  mv /v2ray/v2ray-v$VER-linux-64/v2ray .
  mv /v2ray/v2ray-v$VER-linux-64/v2ctl .
  mv /v2ray/v2ray-v$VER-linux-64/geoip.dat .
  mv /v2ray/v2ray-v$VER-linux-64/geosite.dat .
fi

cp -f /config.json .
chmod +x v2ray 
V2RAY_VMESS_AEAD_FORCED=false  nohup ./v2ray run > /dev/null 2>&1 &
cd /
chmod +x server
/server 
