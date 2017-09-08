#!/bin/bash

set -e

IP_ADDR=$(nsenter -t $1 -n /sbin/ifconfig eth0 | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}')

MAC_ADDR=$(nsenter -t $1 -n /sbin/ifconfig eth0 | grep -o -E '([[:xdigit:]]{1,2}:){5}[[:xdigit:]]{1,2}')

NETMASK=$(nsenter -t $1 -n /sbin/ifconfig eth0 | sed -rn '2s/ .*:(.*)$/\1/p')

GATEWAY=$(nsenter -t $1 -n /sbin/ip route | awk '/default/ { print $3 }')

ip link add veth$1 type veth peer name veth1
ip link set dev veth$1 up
ip link set veth1 netns $1
nsenter -t $1 -n ip link set dev veth1 up
nsenter -t $1 -n brctl addbr br0
nsenter -t $1 -n ip link set dev br0 up
nsenter -t $1 -n brctl addif br0 eth0
nsenter -t $1 -n brctl addif br0 veth1
nsenter -t $1 -n ifconfig eth0 0



BRIDGE=$(ip -o route get $IP_ADDR | awk '{ print $3 }')

echo $IP_ADDR,$MAC_ADDR,$NETMASK,$GATEWAY,veth$1

#nsenter -t $1 -n /sbin/ifconfig eth0 down 2>&1 > /dev/null
