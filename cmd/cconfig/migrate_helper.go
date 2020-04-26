// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"errors"
	"strings"
	"time"

	"github.com/ledisdb/xcodis/models"
	"github.com/ngaut/zkhelper"

	log "github.com/ngaut/logging"

	"github.com/garyburd/redigo/redis"
	_ "github.com/juju/errors"
)

const (
	MIGRATE_TIMEOUT = 30000
)

var ErrGroupMasterNotFound = errors.New("group master not found")
var ErrInvalidAddr = errors.New("invalid addr")

type migrater struct {
	group string
}

func (m *migrater) nextGroup() {
	switch m.group {
	case "KV":
		m.group = "HASH"
	case "HASH":
		m.group = "LIST"
	case "LIST":
		m.group = "SET"
	case "SET":
		m.group = "ZSET"
	case "ZSET":
		m.group = ""
	}
}

// return: success_count, remain_count, error
// slotsmgrt host port timeout slotnum count
func (m *migrater) sendRedisMigrateCmd(c redis.Conn, slotId int, toAddr string) (bool, error) {
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

func (m *migrater) sendLedisMigrateCmd(c redis.Conn, slotId int, toAddr string) (bool, error) {
	addrParts := strings.Split(toAddr, ":")
	if len(addrParts) != 2 {
		return false, ErrInvalidAddr
	}

	count := 10
	num, err := redis.Int(c.Do("xmigratedb", addrParts[0], addrParts[1], m.group, count, slotId, MIGRATE_TIMEOUT))
	if err != nil {
		return false, err
	} else if num < count {
		m.nextGroup()
		return m.group != "", nil
	} else {
		return true, nil
	}
}

func (m *migrater) sendMigrateCmd(c redis.Conn, slotId int, toAddr string) (bool, error) {
	if broker == LedisBroker {
		return m.sendLedisMigrateCmd(c, slotId, toAddr)
	} else {
		return m.sendRedisMigrateCmd(c, slotId, toAddr)
	}
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

	m := new(migrater)
	m.group = "KV"

	remain, err := m.sendMigrateCmd(c, slotId, toMaster.Addr)
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
		remain, err = m.sendMigrateCmd(c, slotId, toMaster.Addr)
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
