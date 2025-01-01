package kvfile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/glycerine/greenpack/msgp"
)

// Magic always there header bytes provide a
// basic sanity check that we are reading our
// expected file format.
var Magic = []byte{0x88, 0xdb, 0x0f, 0xb7, 0x42, 0x64, 0xa6, 0xff}

// ulen is optional msg len written in the 8 bytes after the Magic.
func WriteKVFile(w io.Writer, kv *KVFile, ulen uint64) error {

	// write Magic
	_, err := w.Write(Magic)
	if err != nil {
		return fmt.Errorf("WriteKVFile write magic error: '%v'", err)
	}

	// write ulen; might be 8 bytes of zeros
	var optionalMsgLen [8]byte
	binary.BigEndian.PutUint64(optionalMsgLen[:], ulen)
	_, err = w.Write(optionalMsgLen[:])
	if err != nil {
		return fmt.Errorf("WriteKVFile write msg len error: '%v'", err)
	}

	err = msgp.Encode(w, kv)
	if err != nil {
		return fmt.Errorf("WriteKVFile Encode error: '%v'", err)
	}
	return nil
}

func ReadKVFile(r io.Reader) (kv *KVFile, err error) {

	// read magic
	var magicCheck [8]byte
	_, err = io.ReadFull(r, magicCheck[:])
	if err != nil {
		return nil, fmt.Errorf("ReadKVFile magic check read error: '%v'", err)
	}
	if !bytes.Equal(magicCheck[:], Magic) {
		return nil, fmt.Errorf("ReadKVFile magic check error expected '%x' but got '%x'", Magic, magicCheck)
	}

	kv = &KVFile{}

	var optionalMsgLen [8]byte
	_, err = io.ReadFull(r, optionalMsgLen[:])
	if err != nil {
		return nil, fmt.Errorf("ReadKVFile read msg len error: '%v'", err)
	}
	ulen := binary.BigEndian.Uint64(optionalMsgLen[:])
	_ = ulen // not used atm

	err = msgp.Decode(r, kv)
	return
}
