#!/bin/sh

nohup redis-server ./redis_conf/6380.conf &> ./log/redis_6380.log &
echo "sleep 1s"
sleep 1
tail -n 30 ./log/redis_6380.log

nohup redis-server ./redis_conf/6381.conf &> ./log/redis_6381.log &
echo "sleep 1s"
sleep 1
tail -n 30 ./log/redis_6381.log

