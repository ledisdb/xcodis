# xcodis

Yet another redis proxy based on [codis](https://github.com/wandoulabs/codis)

**Please read codis document first.**

## Why xcodis?

+ Support [LedisDB](https://github.com/siddontang/ledisdb).
+ Support origin Redis, Wandoujia uses a modified version.

## Changes from codis

+ Use db index to represent slot concept in codis, every operations must call `select db` first with a little performance degradation.
+ `DEFAULT_SLOT_NUM` must equal redis/ledisdb databases. 16 is the default for redis, and ledisdb only supports 16 now. (may change later.)
+ Use `scan` + `migrate` in redis for slot migration.
+ Use `xmigrate` + `xmigratedb` in ledisdb for slot migration.
+ Remove dashboard, maybe add later. 
+ Remove slot rebalance feature, maybe add later.
+ Must set a broker in `config.ini`, broker is `ledisdb` or `redis`.

## Thanks

Thanks Wandoujia, codis is a very awesome application.

## Feedback

+ gmail: siddontang@gmail.com
+ skype: siddontang_1
