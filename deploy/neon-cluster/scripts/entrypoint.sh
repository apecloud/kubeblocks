#!/bin/sh
cd /opt/neondatabase-neon
./target/release/neon_local start
./target/release/neon_local pg start main
while true; do
sleep 1000
done
