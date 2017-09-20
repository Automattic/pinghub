#!/bin/bash

# SSH as myself, not root
useradd -m -s /bin/bash -g admin andy
mkdir /home/andy/.ssh
mv /root/.ssh/authorized_keys /home/andy/.ssh/
chown -R andy:admin /home/andy/.ssh
sed -i 's/PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config
sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
service sshd restart
echo 'andy ALL=NOPASSWD:ALL' > /etc/sudoers.d/andy

# Install doctl
curl -L https://github.com/digitalocean/doctl/releases/download/v1.4.0/doctl-1.4.0-linux-amd64.tar.gz | tar zx --directory /usr/local/bin
chmod +x /usr/local/bin/doctl
apt-get update
apt-get install jq -y

# Allow new droplets thru firewall
ufw reset; ufw default deny; ufw allow OpenSSH; ufw enable; while true; do sleep 3; doctl -t 3b40c7071f8df7df404701cf17bd3c127e15b2bb1e1685523561a4952477f243 compute droplet list -o json | jq -c -r '.[].networks.v4[].ip_address' | while read ip; do ufw allow from $ip; done; done &

# Install rethinkdb
source /etc/lsb-release && echo "deb http://download.rethinkdb.com/apt $DISTRIB_CODENAME main" | tee /etc/apt/sources.list.d/rethinkdb.list
wget -qO- https://download.rethinkdb.com/apt/pubkey.gpg | apt-key add -
apt-get update
apt-get install rethinkdb -y
echo "bind=all" > /etc/rethinkdb/instances.d/default.conf
echo "server-tag=tor" >> /etc/rethinkdb/instances.d/default.conf
echo "join=159.203.97.45:29015" >> /etc/rethinkdb/instances.d/default.conf
service rethinkdb restart
