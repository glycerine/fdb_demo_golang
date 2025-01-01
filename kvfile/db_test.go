package kvfile

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	cv "github.com/glycerine/goconvey/convey"
)

var _ = fmt.Printf
var _ = strings.Contains
var _ = time.Time{}
var _ = reflect.DeepEqual

func Test001_fdb_set_and_get_single_keys(t *testing.T) {

	cv.Convey("should be able to set and get single keys in from FDB", t, func() {
		tenant := "jason"
		namespaceName := "kvfile_test"
		db, err := NewDBPool(20, tenant, namespaceName)
		panicOn(err)
		defer db.Close()

		key := []byte("my_first_key")
		val := []byte("my_first_val")
		err = db.WriteKV(key, val)
		panicOn(err)
		val1, err := db.ReadKV(key, nil)
		panicOn(err)
		cv.So(bytes.Equal(val1, val), cv.ShouldBeTrue)

		key2 := []byte("my_second_key")
		val2 := []byte("my_second_val")
		err = db.WriteKV(key2, val2)
		panicOn(err)
		val22, err := db.ReadKV(key2, nil)
		panicOn(err)
		cv.So(bytes.Equal(val22, val2), cv.ShouldBeTrue)

		// clear just one with a ClearRange
		endx := []byte("my_first_key\x00")
		vv("endx = '%x'", endx)

		err = db.ClearRange([]byte("my_first_key"), endx)
		panicOn(err)

		// should not read key. should read key2.
		val1back, err := db.ReadKV(key, nil)
		panicOn(err)
		cv.So(len(val1back), cv.ShouldEqual, 0)

		val222, err := db.ReadKV(key2, nil)
		panicOn(err)
		cv.So(bytes.Equal(val222, val2), cv.ShouldBeTrue)
		vv("val222 = '%v'", string(val222))

		// this should not clear key2, b/c the end point is excluded.

		err = db.ClearRange([]byte("my_first_key"), []byte("my_second_key"))
		panicOn(err)
		val2222, err := db.ReadKV(key2, nil)
		panicOn(err)
		cv.So(bytes.Equal(val2222, val2), cv.ShouldBeTrue)
		vv("val222 = '%v'", string(val2222))

		// we should have just my_second_key left in this range

		kvs, err := db.GetRange([]byte("my_first_key"), []byte("my_second_key\x00"), nil)
		panicOn(err)
		for i := range kvs {
			vv("kvs['%v'] = '%v'", string(kvs[i].Key), string(kvs[i].Value))
		}
		cv.So(len(kvs), cv.ShouldEqual, 1)

		// and clear the other one

		err = db.ClearRange([]byte("my_first_key"), []byte("my_second_key\x00"))
		panicOn(err)
		val22222, err := db.ReadKV(key2, nil)
		panicOn(err)
		cv.So(bytes.Equal(val22222, []byte{}), cv.ShouldBeTrue)
		vv("val22222 = '%v'", string(val22222))

		//panicOn(db.Clear(key))
	})
}

func Test002_read_kvfile_into_database(t *testing.T) {

	cv.Convey("KVFile should import and export from database", t, func() {
		tenant := "jason"
		namespaceName := "kvfile_test"
		db, err := NewDBPool(8, tenant, namespaceName)
		panicOn(err)
		defer db.Close()

		kv := NewKVFile()

		/*
			kv.AddKVStrings("key number 1", "val number 1")
			kv.AddKVStrings("key number 2", "val number 2")
			kv.AddKVStrings("key number 3", "val number 3")
		*/
		big := make([]byte, 100_000-1)
		for i := 0; i < 3101; i++ {
			bigkey := fmt.Sprintf("big %v", i)
			kv.AddKV(bigkey, big)
		}

		path := "test002path"

		t0 := time.Now()
		err = db.ImportKVFileToDBPath(path, kv)
		panicOn(err)
		t1 := time.Now()

		kv2, err := db.ReadKVFileDBPath(path)
		panicOn(err)
		t2 := time.Now()

		totalBytes := 0
		for i := range kv2.Payload {
			nb := len(kv2.Payload[i].Val)
			//vv("kv2 key[%v] = '%v' -> val len %v", i, string(kv2.Payload[i].Key), nb)
			totalBytes += nb
		}
		vv("total number of bytes read back = %v", totalBytes)

		mb := float64(totalBytes) / (1 << 20)

		e0 := t1.Sub(t0)
		sec0 := (float64(e0) / float64(time.Second))
		vv("write acheived: %0.2f MB/sec in %v", mb/sec0, e0)

		e1 := t2.Sub(t1)
		sec1 := (float64(e1) / float64(time.Second))
		vv("read acheived: %0.2f MB/sec in %v", mb/sec1, e1)

		// before using the pool in parallel:

		// total number of bytes read back = 30999690
		// write acheived: 180.30 MB/sec in 163.97326ms
		// read acheived: 196.62 MB/sec in 150.359619ms

		// db_test.go:128 2024-12-31T19:28:28.595-06:00 total number of bytes read back = 309_996_900

		// write with pool of only 1:
		// db_test.go:134 2024-12-31T19:28:28.595-06:00 write acheived: 198.43 MB/sec in 1.489907282s

		// pool of 2 (same size batch)
		//db_test.go:134 2024-12-31T20:17:25.348-06:00 write acheived: 263.11 MB/sec in 1.124004724s

		// pool of 4
		//db_test.go:134 2024-12-31T20:20:13.267-06:00 write acheived: 313.57 MB/sec in 943.112611ms

		//cv.So(reflect.DeepEqual(kv, kv2), cv.ShouldBeTrue)

		panicOn(db.DeleteDBPath(path))
	})
}
