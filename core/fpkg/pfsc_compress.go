package fpkg

/*
#cgo LDFLAGS: -lz
#include <zlib.h>
#include <string.h>

// pfsc_compress_block compresses a 64 KiB block using zlib with windowBits=12
// (4 KiB window), matching the PS4 kernel's PFSC decompressor (flatz/pkg_pfs_tool).
//
// Returns compressed size (Z_OK), or -1 if the output buffer is too small
// (which means compression didn't help — caller should store uncompressed).
static int pfsc_compress_block(const unsigned char *in, unsigned long in_len,
                               unsigned char *out, unsigned long out_cap,
                               unsigned long *out_len)
{
    z_stream s;
    memset(&s, 0, sizeof(s));

    // windowBits=12 (zlib format), level=6, memLevel=8
    if (deflateInit2(&s, 6, Z_DEFLATED, 12, 8, Z_DEFAULT_STRATEGY) != Z_OK)
        return -2;

    s.next_in   = (Bytef *)in;
    s.avail_in  = in_len;
    s.next_out  = (Bytef *)out;
    s.avail_out = out_cap;

    int rc = deflate(&s, Z_FINISH);
    *out_len = s.total_out;
    deflateEnd(&s);

    if (rc == Z_STREAM_END)
        return 0;
    return rc;  // Z_OK or Z_BUF_ERROR (-5)
}
*/
import "C"

import (
	"unsafe"
)

// compressPFSCBlock tries to compress a 64 KiB block with windowBits=12.
// Returns the compressed data (zlib format) or nil if compression didn't help.
func compressPFSCBlock(block []byte) []byte {
	if len(block) == 0 {
		return nil
	}
	// Output buffer: worst case zlib overhead is ~0.1% + 11 bytes
	outCap := len(block) + len(block)/1000 + 64
	outBuf := make([]byte, outCap)
	outLen := C.ulong(0)

	rc := C.pfsc_compress_block(
		(*C.uchar)(unsafe.Pointer(&block[0])),
		C.ulong(len(block)),
		(*C.uchar)(unsafe.Pointer(&outBuf[0])),
		C.ulong(outCap),
		&outLen,
	)

	if rc != 0 || int(outLen) >= len(block) {
		return nil // compression failed or didn't help
	}
	return outBuf[:int(outLen)]
}
