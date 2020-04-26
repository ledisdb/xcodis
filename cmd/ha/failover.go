package main

import (
	"fmt"

	"github.com/ledisdb/redis-failover/failover"
	"github.com/ledisdb/xcodis/models"
	"github.com/ledisdb/xcodis/utils"
	log "github.com/ngaut/logging"
	"github.com/ngaut/zkhelper"
)

func BeforePromote(oldMaster string) error {
	conn, _ := zkhelper.ConnectToZk(*zkAddr)
	defer conn.Close()

	groups, err := models.ServerGroups(conn, *productName)
	if err != nil {
		log.Errorf("get server groups error %v, give up failover", err)
		return failover.ErrGiveupFailover
	}

	found := false
	for _, group := range groups {
		for _, server := range group.Servers {
			if server.Addr == oldMaster {
				found = true
				break
			}
		}
	}

	if !found {
		log.Errorf("can not find %s in any groups, give up failover", oldMaster)
		return failover.ErrGiveupFailover
	}

	return nil
}

func Promote(oldMaster string, newMaster string) error {
	conn, _ := zkhelper.ConnectToZk(*zkAddr)
	defer conn.Close()

	groups, err := models.ServerGroups(conn, *productName)
	if err != nil {
		return err
	}

	var groupId int
	found := false
	for _, group := range groups {
		for _, server := range group.Servers {
			if server.Addr == oldMaster {
				groupId = group.Id
				found = true
				break
			}
		}
	}

	if !found {
		return fmt.Errorf("can not find %s in any groups", oldMaster)
	}

	lock := utils.GetZkLock(conn, *productName)
	lock.Lock(fmt.Sprintf("promote server %+v", newMaster))
	defer func() {
		err := lock.Unlock()
		if err != nil {
			log.Warning(err)
		}
	}()

	group, err := models.GetGroup(conn, *productName, groupId)
	if err != nil {
		return err
	}
	err = group.Promote(conn, newMaster)
	if err != nil {
		return err
	}

	return nil
}
