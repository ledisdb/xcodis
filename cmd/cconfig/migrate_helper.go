// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"errors"
	"strings"
	"time"

	"github.com/ngaut/zkhelper"
	"github.com/siddontang/xcodis/models"

	log "github.com/ngaut/logging"

	"github.com/garyburd/redigo/redis"
	_ "github.com/juju/errors"
)

const (
	MIGRATE_TIMEOUT = 30000
)

var ErrGroupMasterNotFound = errors.New("group master not found")
var ErrInvalidAddr = errors.New("invalid addr")

// return: success_count, remain_count, error
// slotsmgrt host port timeout slotnum count
func sendRedisMigrateCmd(c redis.Conn, slotId int, toAddr string) (bool, error) {
	addrParts := strings.Split(toAddr, ":")
	if len(addrParts) != 2 {
		return false, ErrInvalidAddr
	}

	//use scan and migrate
	reply, err := redis.MultiBulk(c.Do("scan", 0))
	if err != nil {
		return false, err
	}

	var next string
	var keys []interface{}

	if _, err := redis.Scan(reply, &next, &keys); err != nil {
		return false, err
	}

	for _, key := range keys {
		if _, err := c.Do("migrate", addrParts[0], addrParts[1], key, slotId, MIGRATE_TIMEOUT); err != nil {
			//todo, try del if key exists
			return false, err
		}
	}

	return next != "0", nil
}

var ErrStopMigrateByUser = errors.New("migration stop by user")

func MigrateSingleSlot(zkConn zkhelper.Conn, slotId, fromGroup, toGroup int, delay int, stopChan <-chan struct{}) error {
	groupFrom, err := models.GetGroup(zkConn, productName, fromGroup)
	if err != nil {
		return err
	}
	groupTo, err := models.GetGroup(zkConn, productName, toGroup)
	if err != nil {
		return err
	}

	fromMaster, err := groupFrom.Master(zkConn)
	if err != nil {
		return err
	}

	toMaster, err := groupTo.Master(zkConn)
	if err != nil {
		return err
	}

	if fromMaster == nil || toMaster == nil {
		return ErrGroupMasterNotFound
	}

	c, err := redis.Dial("tcp", fromMaster.Addr)
	if err != nil {
		return err
	}

	defer c.Close()

	if ok, err := redis.String(c.Do("select", slotId)); err != nil {
		return err
	} else if ok != "OK" {
		return errors.New(ok)
	}

	remain, err := sendRedisMigrateCmd(c, slotId, toMaster.Addr)
	if err != nil {
		return err
	}

	num := 0
	for remain {
		if delay > 0 {
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
		if stopChan != nil {
			select {
			case <-stopChan:
				return ErrStopMigrateByUser
			default:
			}
		}
		remain, err = sendRedisMigrateCmd(c, slotId, toMaster.Addr)
		if num%500 == 0 && remain {
			log.Infof("still migrating")
		}
		num++
		if err != nil {
			return err
		}
	}
	return nil
}
