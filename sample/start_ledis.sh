#!/bin/sh

rm -rf /tmp/xcodis_test

nohup ledis-server -data_dir=/tmp/xcodis_test/6380 -db_name=memory -addr=127.0.0.1:6380 &> ./log/ledis_6380.log &
echo "sleep 1s"
sleep 1
tail -n 30 ./log/ledis_6380.log

nohup ledis-server -data_dir=/tmp/xcodis_test/6381 -db_name=memory -addr=127.0.0.1:6381 &> ./log/ledis_6381.log &
echo "sleep 1s"
sleep 1
tail -n 30 ./log/ledis_6381.log
