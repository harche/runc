#!/bin/bash

IP_ADDR=$(nsenter --net=$1 /sbin/ifconfig eth0 | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}')
MAC_ADDR=$(nsenter --net=$1 /sbin/ifconfig eth0 | grep -o -E '([[:xdigit:]]{1,2}:){5}[[:xdigit:]]{1,2}')
NETMASK=$(nsenter --net=$1 /sbin/ifconfig eth0 | sed -rn '2s/ .*:(.*)$/\1/p')
GATEWAY=$(nsenter --net=$1 /sbin/ip route | awk '/default/ { print $3 }')

echo $IP_ADDR $MAC_ADDR $NETMASK $GATEWAY
