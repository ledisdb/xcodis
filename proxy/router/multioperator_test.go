// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"reflect"
	"testing"

	"github.com/alicebob/miniredis"
)

var redisrv *miniredis.Miniredis

func TestMgetResults(t *testing.T) {
	redisrv, err := miniredis.Run()
	if err != nil {
		t.Fatal("can not run miniredis")
	}
	defer redisrv.Close()

	moper := NewMultiOperator(redisrv.Addr())
	redisrv.Set("a", "a")
	redisrv.Set("b", "b")
	redisrv.Set("c", "c")
	ay, err := moper.mgetResults(&MulOp{
		op: "mget",
		keys: [][]byte{[]byte("a"),
			[]byte("b"), []byte("c"), []byte("x")}})
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(ay, []interface{}{[]byte("a"), []byte("b"), []byte("c"), nil}) {
		t.Fatalf("%v", ay)
	}

	_, err = moper.mgetResults(&MulOp{
		op: "mget",
		keys: [][]byte{[]byte("x"),
			[]byte("c"), []byte("x")}})
	if err != nil {
		t.Error(err)
	}

	_, err = moper.mgetResults(&MulOp{
		op: "mget",
		keys: [][]byte{[]byte("x"),
			[]byte("y"), []byte("x")}})
	if err != nil {
		t.Error(err)
	}
}

func TestDeltResults(t *testing.T) {
	redisrv, err := miniredis.Run()
	if err != nil {
		t.Fatal("can not run miniredis")
	}
	defer redisrv.Close()

	moper := NewMultiOperator(redisrv.Addr())
	redisrv.Set("a", "a")
	redisrv.Set("b", "b")
	redisrv.Set("c", "c")
	buf, err := moper.delResults(&MulOp{
		op: "del",
		keys: [][]byte{[]byte("a"),
			[]byte("b"), []byte("c")}})
	if err != nil {
		t.Error(err)
	}

	if buf != 3 {
		t.Fatalf("%d not match 3", buf)
	}
}
