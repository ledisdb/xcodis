# xcodis

Yet another redis proxy based on [codis](https://github.com/wandoulabs/codis)

**Please read codis document first. [here](https://github.com/wandoulabs/codis/blob/master/doc)** 

## Install 

+ Install Go and set your [GOPATH](https://golang.org/doc/code.html#GOPATH)
+ Install godep `go get github.com/tools/godep`
+ `go get github.com/siddontang/xcodis`, it will print some messages in console, skip it. :-)
+ cd `$GOPATH/src/github.com/siddontang/xcodis`
+ `make`

## Why xcodis?

+ Supports [LedisDB](https://github.com/siddontang/ledisdb).
+ Supports origin Redis, codis uses a modified version.

## Changes from codis

+ Uses db index to represent slot concept in codis, every operations must call `select db` first with a little performance degradation.
+ `DEFAULT_SLOT_NUM` must equal redis/ledisdb databases. 16 is the default for redis, and ledisdb only supports 16 now. (may change later.)
+ Uses `scan` + `migrate` in redis for slot migration.
+ Uses `xmigrate` + `xmigratedb` in ledisdb for slot migration.
+ Removes dashboard. 
+ Removes slot rebalance feature.
+ Must set a broker in `config.ini`, broker is `ledisdb` or `redis`.
+ Uses a white command list for ledisdb.
+ Not support atomic tag migration.
+ Not support lua for ledisdb.

## Todo

+ Tidy up some ugly codes I added. >_<

## Thanks

Thanks Wandoujia, codis is a very awesome application.

## Feedback

+ gmail: siddontang@gmail.com
