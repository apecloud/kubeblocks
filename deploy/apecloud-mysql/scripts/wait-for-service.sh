#!/bin/sh
echo "wait for $1"
while ! nc -z $2 $3
do sleep 1
printf "-"
done
echo -e "  >> $1 has started"
