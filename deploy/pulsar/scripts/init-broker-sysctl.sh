#!/bin/sh
sysctl -w net.ipv4.tcp_keepalive_time=1 && sysctl -w net.ipv4.tcp_keepalive_intvl=11 && sysctl -w net.ipv4.tcp_keepalive_probes=3