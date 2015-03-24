// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/siddontang/goredis"
)

type session struct {
	*goredis.Conn

	CreateAt time.Time
	Ops      int64
}

var (
	keyFun = make(map[string]funGetKeys)
)

func init() {
	for _, v := range thridAsKeyTbl {
		keyFun[v] = thridAsKey
	}
}

type funGetKeys func([][]byte) ([][]byte, error)

var thridAsKeyTbl = []string{"ZINTERSTORE", "ZUNIONSTORE", "EVAL", "EVALSHA"}

//todo: overflow
func btoi(b []byte) (int, error) {
	n := 0
	sign := 1
	for i := uint8(0); i < uint8(len(b)); i++ {
		if i == 0 && b[i] == '-' {
			if len(b) == 1 {
				return 0, errors.Errorf("Invalid number %s", string(b))
			}
			sign = -1
			continue
		}

		if b[i] >= '0' && b[i] <= '9' {
			if i > 0 {
				n *= 10
			}
			n += int(b[i]) - '0'
			continue
		}

		return 0, errors.Errorf("Invalid number %s", string(b))
	}

	return sign * n, nil
}

func thridAsKey(keys [][]byte) ([][]byte, error) {
	if len(keys) < 3 { //if EVAL with no key
		return [][]byte{[]byte("fakeKey")}, nil
	}

	numKeys, err := btoi(keys[1])
	if err != nil {
		return nil, errors.Trace(err)
	}

	stop := 2 + numKeys
	if stop >= len(keys) {
		stop = len(keys)
	}
	keys = keys[2:stop]

	return keys, nil
}

func (s *session) ReadRequest() (op string, keys [][]byte, err error) {
	keys, err = s.ReceiveRequest()
	if err != nil {
		return
	}

	if len(keys) == 0 {
		err = errors.Errorf("invalid command request")
		return
	}

	op = strings.ToUpper(string(keys[0]))
	keys = keys[1:]

	f, ok := keyFun[op]
	if !ok {
		return op, keys, nil
	}

	keys, err = f(keys)
	return op, keys, errors.Trace(err)
}
