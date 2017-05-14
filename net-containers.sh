#!/bin/bash

[[ $# -ne 2 ]] && { echo "Usage: $0 mode count"; exit 1; }

mode=$1
count=$2

if [[ $mode == "start" ]]; then
	parallel docker run --rm -d --name net-{} --label="plumber.network.mode=macvlan" --label="plumber.network.interfacename=eth0" --label="plumber.network.vlanid=3134" --net=none --cap-add NET_ADMIN ictu/pipes:dhcpcd ::: $(seq 1 $count)
fi

if [[ $mode == "check" ]]; then
	for i in $(seq 1 $count); do
    docker exec net-$i ifconfig eth0 | grep -A1 eth0;
  done
fi

if [[ $mode == "stop" ]]; then
	for i in $(seq 1 $count); do
    docker kill net-$i;
  done
fi
