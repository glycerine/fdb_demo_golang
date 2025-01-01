package kvfile

import (
	//"bytes"
	//"fmt"
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/apple/foundationdb/bindings/go/src/fdb/subspace"
	"math"
	"strings"
	"sync"
	//"sync/atomic"
)

var _ subspace.Subspace

type DB struct {
	db        *fdb.Database
	namespace string
	tenant    string
	tenNS     string
	ten       fdb.Tenant
}

func (p *DB) Close() {
	p.db.Close()
}

type DBPool struct {
	pool []*DB
	next int
	mut  sync.Mutex
}

func (p *DBPool) Next() *DB {
	p.mut.Lock()
	defer p.mut.Unlock()

	i := p.next
	p.next = (p.next + 1) % len(p.pool)
	//vv("issuing db i=%v", i)
	return p.pool[i]
}

func (p *DBPool) Close() {
	for _, db := range p.pool {
		db.Close()
	}
}

func NewDBPool(numDB int, tenant, namespaceName string) (p *DBPool, err error) {
	p = &DBPool{}
	for range numDB {
		db, err := newDB(tenant, namespaceName)
		if err != nil {
			p.Close()
			return nil, err
		}
		p.pool = append(p.pool, db)
	}
	return p, nil
}

type namespace struct {
	name   string
	prefix []byte // same as name but ready to go.
}

// New returns a new DB. Users should use NewDBPool() instead.
func newDB(tenant, namespaceName string) (*DB, error) {
	err := fdb.APIVersion(730)
	if err != nil {
		return nil, err
	}
	db, err := fdb.OpenDefault()
	if err != nil {
		return nil, err
	}

	if !strings.HasSuffix(namespaceName, "/") {
		namespaceName += "/"
	}
	// levage the tenant mechanism for namespace isolation on the backend.
	tenNS := tenant + "/" + namespaceName

	// oddly we could not get going by trying to Open() first.
	// But CreateTenant() first does seem to work.
	err = db.CreateTenant(fdb.Key(tenNS))
	if err == nil {
		// rarely seen, but we have to create before open will work, it seems.
		vv("created tenant/namespace just fine: '%v'", tenNS)
	} else {
		if strings.Contains(err.Error(), "tenant with the given name already exists") {
			// ignore, expected.
		} else {
			vv("error creating tenant/namespace: '%v' was '%v' ... will try to open", tenNS, err)
		}
	}
	ten, err := db.OpenTenant(fdb.Key(tenNS))
	panicOn(err)
	//vv("opened tenant/namespace: '%v'", tenNS)

	// formal subspace would work, but would be
	// quite a bit more elaborate
	// than we need at the moment.
	//subspace := subspace.FromBytes([]byte(prefix))

	return &DB{
		db:        &db,
		namespace: namespaceName,
		tenant:    tenant,
		ten:       ten,
		tenNS:     tenNS,
	}, nil
}

func (dbp *DBPool) WriteKV(key, val []byte) error {
	if len(key) == 0 {
		return nil
	}

	db := dbp.Next()

	_, err := db.ten.Transact(func(tr fdb.Transaction) (interface{}, error) {
		key1 := fdb.Key(key)
		tr.Set(key1, val)
		return nil, nil
	})

	return err
}

func (dbp *DBPool) ReadKV(key, valbuf []byte) (val []byte, err error) {
	if len(key) == 0 {
		return nil, nil
	}
	db := dbp.Next()

	var vali any
	vali, err = db.ten.Transact(func(tr fdb.Transaction) (interface{}, error) {
		key1 := fdb.Key(key)
		return tr.Get(fdb.Key(key1)).Get()
	})

	if vali != nil {
		val = vali.([]byte)
	}
	return
}

// Clear deletes a single key. See ClearRange below to delete a range.
func (dbp *DBPool) Clear(key []byte) (err error) {
	if len(key) == 0 {
		return nil
	}
	db := dbp.Next()
	_, err = db.ten.Transact(func(tr fdb.Transaction) (interface{}, error) {
		key1 := fdb.Key(key)
		tr.Clear(key1)
		return nil, nil
	})
	return
}

/* KeyRange satisfies the fdb.ExactRange interface

// KeyRange is an ExactRange constructed from a pair of KeyConvertibles. Note
// that the default zero-value of KeyRange specifies an empty range before all
// keys in the database.
type KeyRange struct {
	// The (inclusive) beginning of the range
	Begin KeyConvertible

	// The (exclusive) end of the range
	End KeyConvertible
}
*/

func (dbp *DBPool) ClearRange(beg, endx []byte) (err error) {

	db := dbp.Next()

	// isolate to our namespace
	var kr fdb.KeyRange
	kr.Begin = fdb.Key(beg)
	kr.End = fdb.Key(endx)

	_, err = db.ten.Transact(func(tr fdb.Transaction) (interface{}, error) {
		tr.ClearRange(kr)
		return nil, nil
	})
	return
}

func (dbp *DBPool) GetRange(beg, endx []byte, options *fdb.RangeOptions) (kvs []fdb.KeyValue, err error) {

	db := dbp.Next()

	if options == nil {
		options = &fdb.RangeOptions{
			Mode: fdb.StreamingModeWantAll,
		}
	}

	// isolate to our namespace
	var kr fdb.KeyRange
	kr.Begin = fdb.Key(beg)
	kr.End = fdb.Key(endx)

	_, err = db.ten.ReadTransact(func(tr fdb.ReadTransaction) (interface{}, error) {

		kvs, err = tr.GetRange(kr, *options).GetSliceWithError()
		if err != nil {
			kvs = nil
			return nil, err
		}
		for i := range kvs {
			kvs[i].Key = kvs[i].Key
		}
		return nil, nil
	})
	return
}

func (dbp *DBPool) ImportKVFileToDBPath(path string, kv *KVFile) (err error) {

	pairsToWrite := len(kv.Payload)
	numDBToUse := pairsToWrite
	if numDBToUse > len(dbp.pool) {
		numDBToUse = len(dbp.pool)
	}
	pairsPerDB := int(math.Ceil(float64(pairsToWrite) / float64(numDBToUse)))

	// assign jobs to goroutines in contiguous spans.
	jobs := make([]*KVFile, numDBToUse)
	next := 0
	done := false
	for j, job := range jobs {
		job = &KVFile{}
		jobs[j] = job
		last := next + pairsPerDB
		if last > len(kv.Payload) {
			last = len(kv.Payload)
			done = true
		}
		job.Payload = kv.Payload[next:last]
		vv("out of %v pairs, kvs/jobs %v got [%v: %v)", pairsToWrite, j, next, last)
		if done {
			break
		}
		next += pairsPerDB
	}

	// goroutines will report errors here.
	errs := make([]error, numDBToUse)

	var wg sync.WaitGroup
	wg.Add(numDBToUse)

	for kdb := range numDBToUse {

		go func(kdb int, kv *KVFile) (err error) {
			vv("goro %v starting with %v jobs", kdb, len(kv.Payload))
			defer func() {
				wg.Done()
				errs[kdb] = err
				vv("goro %v done", kdb)
			}()
			db := dbp.Next()

			klen := 10_000
			k := make([]byte, klen)
			n := len(path) + 1
			copy(k, []byte(path+"#"))

			npair := len(kv.Payload)

			for i := 0; i < npair; {

				_, err = db.ten.Transact(func(tr fdb.Transaction) (interface{}, error) {

					bytesUsed := 0
					scanme := kv.Payload[i:]
					for _, pair := range scanme {
						m := len(pair.Key)
						if m+n > klen {
							// buffer too small. enlarge it.
							klen = klen*2 + n + m
							k = make([]byte, klen)
							copy(k, []byte(path+"#"))
						}
						copy(k[n:], pair.Key)
						key1 := fdb.Key(k[:n+m])
						tr.Set(key1, pair.Val)
						i++

						bytesUsed += len(key1) + len(pair.Val)
						if bytesUsed > 9_000_000 {
							//vv("near the 10MB transaction limit, start another transaction. i = %v", i)
							return nil, nil
						}
					}
					return nil, nil
				})
				if err != nil {
					return err
				}
			} // end for i over npair

			return err
		}(kdb, jobs[kdb])
	}
	wg.Wait()
	// return the first error we find, if any
	for i := range errs {
		if errs[i] != nil {
			return errs[i]
		}
	}
	return nil
}

func (dbp *DBPool) ReadKVFileDBPath(path string) (kv *KVFile, err error) {

	db := dbp.Next()

	options := &fdb.RangeOptions{
		Mode: fdb.StreamingModeWantAll,
	}

	kv = &KVFile{}
	var kr fdb.KeyRange
	kr.Begin = fdb.Key(path + "#")
	kr.End = fdb.Key(path + "$") // "$" comes just after "#"

	prefix := len(path) + 1

	_, err = db.ten.ReadTransact(func(tr fdb.ReadTransaction) (interface{}, error) {

		kvs, err := tr.GetRange(kr, *options).GetSliceWithError() // blocked here
		if err != nil {
			return nil, err
		}
		for i := range kvs {
			kv.Payload = append(kv.Payload, &KV{
				Key: kvs[i].Key[prefix:],
				Val: kvs[i].Value,
			})
		}
		return nil, nil
	})
	return
}

func (dbp *DBPool) DeleteDBPath(path string) (err error) {

	db := dbp.Next()

	var kr fdb.KeyRange
	kr.Begin = fdb.Key(path + "#")
	kr.End = fdb.Key(path + "$") // "$" comes just after "#"

	_, err = db.ten.Transact(func(tr fdb.Transaction) (interface{}, error) {
		tr.ClearRange(kr)
		return nil, nil
	})
	return
}

/*
// KeyValue represents a single key-value pair in the database.
type KeyValue struct {
	Key   Key
	Value []byte
}
*/
