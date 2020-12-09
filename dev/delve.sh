#!/bin/bash
killall __debug_bin
killall dlv
#PORT_PID=$!
#echo $PORT_PID
trap 'killall $(jobs -p)' EXIT
make delve
