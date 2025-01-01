package kvfile

//go:generate greenpack

type KV struct {
	Key []byte `zid:"0"`
	Val []byte `zid:"1"`
}

type KVFile struct {
	Payload []*KV `zid:"0"`
}

func NewKVFile() *KVFile {
	return &KVFile{}
}

func (f *KVFile) AddKVStrings(k, v string) {
	kv := &KV{
		Key: []byte(k),
		Val: []byte(v),
	}
	f.Payload = append(f.Payload, kv)
}

func (f *KVFile) AddKV(k string, v []byte) {
	kv := &KV{
		Key: []byte(k),
		Val: v,
	}
	f.Payload = append(f.Payload, kv)
}

func (f *KVFile) AddKVBytes(k, v []byte) {
	kv := &KV{
		Key: k,
		Val: v,
	}
	f.Payload = append(f.Payload, kv)
}
