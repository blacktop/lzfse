package lzfse

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

type Magic uint32

const (
	LZFSE_NO_BLOCK_MAGIC             Magic = 0
	LZFSE_ENDOFSTREAM_BLOCK_MAGIC    Magic = 0x24787662
	LZFSE_UNCOMPRESSED_BLOCK_MAGIC   Magic = 0x2d787662
	LZFSE_COMPRESSEDV1_BLOCK_MAGIC   Magic = 0x31787662
	LZFSE_COMPRESSEDV2_BLOCK_MAGIC   Magic = 0x32787662
	LZFSE_COMPRESSEDLZVN_BLOCK_MAGIC Magic = 0x6e787662
	INVALID                                = 0xdeadbeef
)

type decompressor struct {
	r       *cachedReader
	w       *cachedWriter
	payload io.Reader
}

func decodeUncompressedBlock(r *cachedReader, w *cachedWriter) (err error) {
	var n_raw_bytes uint32
	if err = binary.Read(r, binary.LittleEndian, &n_raw_bytes); err == nil {
		_, err = io.CopyN(w, r, int64(n_raw_bytes))
	}
	return
}

func readBlockMagic(r io.Reader) (magic Magic, err error) {
	err = binary.Read(r, binary.LittleEndian, &magic)
	return
}

type blockHandler func(*cachedReader, *cachedWriter) error

func (d *decompressor) handleBlock(handler blockHandler) (Magic, error) {
	if err := handler(d.r, d.w); err != nil {
		return INVALID, err
	}

	return readBlockMagic(d.r)
}

func (d *decompressor) Read(b []byte) (int, error) {
	if payload, err := d.decompressedPayload(); err == nil {
		return payload.Read(b)
	} else {
		return 0, err
	}
}

func (d *decompressor) decompressedPayload() (io.Reader, error) {
	var err error
	if d.payload == nil {
		d.payload, err = d.decompressAll()
	}
	return d.payload, err
}

func (d *decompressor) decompressAll() (io.Reader, error) {
	var err error
	magic := LZFSE_NO_BLOCK_MAGIC

	for err == nil {
		switch magic {
		case LZFSE_NO_BLOCK_MAGIC:
			magic, err = readBlockMagic(d.r)
		case LZFSE_UNCOMPRESSED_BLOCK_MAGIC:
			magic, err = d.handleBlock(decodeUncompressedBlock)
		case LZFSE_COMPRESSEDV1_BLOCK_MAGIC:
			magic, err = d.handleBlock(decodeCompressedV1Block)
		case LZFSE_COMPRESSEDV2_BLOCK_MAGIC:
			magic, err = d.handleBlock(decodeCompressedV2Block)
		case LZFSE_COMPRESSEDLZVN_BLOCK_MAGIC:
			magic, err = d.handleBlock(decodeLZVNBlock)
		case LZFSE_ENDOFSTREAM_BLOCK_MAGIC:
			magic, err = LZFSE_ENDOFSTREAM_BLOCK_MAGIC, io.EOF
		default:
			magic, err = INVALID, fmt.Errorf("Bad magic")
		}
	}

	if err == io.EOF {
		// @@@ try just reading from it.. not sure if that works correctly.
		return bytes.NewReader(d.w.Bytes()), nil
	}

	return nil, err
}

func NewReader(r io.Reader) *decompressor {
	d := &decompressor{
		r: newCachedReader(r),
		w: newCachedWriter(),
	}

	return d
}
