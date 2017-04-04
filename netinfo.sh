#!/bin/bash

IP_ADDR=$(nsenter --net=$1 /sbin/ifconfig eth0 | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}')
if [ -z "$IP_ADDR" ]
then
   exit 1
fi

MAC_ADDR=$(nsenter --net=$1 /sbin/ifconfig eth0 | grep -o -E '([[:xdigit:]]{1,2}:){5}[[:xdigit:]]{1,2}')
if [ -z "$MAC_ADDR" ]
then
   exit 1
fi

NETMASK=$(nsenter --net=$1 /sbin/ifconfig eth0 | sed -rn '2s/ .*:(.*)$/\1/p')
if [ -z "$NETMASK" ]
then
   exit 1
fi

GATEWAY=$(nsenter --net=$1 /sbin/ip route | awk '/default/ { print $3 }')
if [ -z "$GATEWAY" ]
then
   exit 1
fi

echo $IP_ADDR,$MAC_ADDR,$NETMASK,$GATEWAY

nsenter --net=$1 /sbin/ifconfig eth0 down 2>&1 > /dev/null
