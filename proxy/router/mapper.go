// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"bytes"
	"hash/crc32"

	"fmt"
	// "github.com/siddontang/xcodis/models"
)

const (
	HASHTAG_START = '{'
	HASHTAG_END   = '}'
)

func mapKey2Slot(key []byte) int {
	hashKey := key
	//hash tag support
	htagStart := bytes.IndexByte(key, HASHTAG_START)
	if htagStart >= 0 {
		htagEnd := bytes.IndexByte(key[htagStart:], HASHTAG_END)
		if htagEnd >= 0 {
			hashKey = key[htagStart+1 : htagStart+htagEnd]
		}
	}

	return int(crc32.ChecksumIEEE(hashKey) % uint32(slot_num))
}

func checkKeysInSameSlot(keys [][]byte) (int, error) {
	slot := -1

	for _, key := range keys {
		s := mapKey2Slot(key)
		if slot == -1 {
			slot = s
		} else if slot != s {
			return -1, fmt.Errorf("keys not in same slot")
		}
	}

	return slot, nil
}
