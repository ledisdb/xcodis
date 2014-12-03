#!/bin/sh
echo "slots initializing..."
../bin/codis-config -c config.ini slot init
echo "done"

echo "set slot ranges to server groups..."
../bin/codis-config -c config.ini slot range-set 0 7 1 online
../bin/codis-config -c config.ini slot range-set 8 15 2 online
echo "done"

