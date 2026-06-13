package fpkg

// This file ports the cryptographic primitives from LibOrbisPkg/Util/Crypto.cs,
// LibOrbisPkg/Util/XtsBlockTransform.cs, and LibOrbisPkg/Util/MersenneTwister.cs.
// All operations use Go stdlib only — no external dependencies.

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
)

// ---------------------------------------------------------------------------
// SHA-256 helpers
// ---------------------------------------------------------------------------

// Sha256 returns the SHA-256 hash of data.
func Sha256(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// HmacSha256 returns HMAC-SHA-256 of data using key.
func HmacSha256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// ---------------------------------------------------------------------------
// PKG key derivation
// ---------------------------------------------------------------------------

// ComputeKeys derives a package key for the given content ID, passcode, and index.
// The EKPFS is index 1. Ported from Crypto.ComputeKeys.
func ComputeKeys(contentID, passcode string, index uint32) []byte {
	if len(contentID) != 36 {
		panic("fpkg: Content ID must be 36 characters")
	}
	if len(passcode) != 32 {
		panic("fpkg: Passcode must be 32 characters")
	}

	// SHA256(index_be) || SHA256(contentID_padded_48) || passcode_bytes
	data := make([]byte, 96)

	idxBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(idxBuf, index)
	copy(data[0:32], Sha256(idxBuf))

	paddedID := make([]byte, 48)
	copy(paddedID, contentID)
	copy(data[32:64], Sha256(paddedID))

	copy(data[64:96], []byte(passcode))

	return Sha256(data)
}

// ---------------------------------------------------------------------------
// PFS key generation (HMAC-based)
// ---------------------------------------------------------------------------

// PfsGenCryptoKey derives a PFS crypto key from ekpfs, seed, and index.
// Ported from Crypto.PfsGenCryptoKey.
func PfsGenCryptoKey(ekpfs, seed []byte, index uint32) []byte {
	d := make([]byte, 4+len(seed))
	binary.LittleEndian.PutUint32(d[0:4], index) // C# BitConverter.GetBytes is LE
	copy(d[4:], seed)
	return HmacSha256(ekpfs, d)
}

// PfsGenEncKey generates a (tweakKey, dataKey) pair for AES-XTS.
// Ported from Crypto.PfsGenEncKey.
func PfsGenEncKey(ekpfs, seed []byte) (tweakKey, dataKey []byte) {
	encKey := PfsGenCryptoKey(ekpfs, seed, 1)
	tweakKey = make([]byte, 16)
	dataKey = make([]byte, 16)
	copy(tweakKey, encKey[0:16])
	copy(dataKey, encKey[16:32])
	return tweakKey, dataKey
}

// PfsGenSignKey generates a PFS signing key.
// Ported from Crypto.PfsGenSignKey.
func PfsGenSignKey(ekpfs, seed []byte) []byte {
	return PfsGenCryptoKey(ekpfs, seed, 2)
}

// ---------------------------------------------------------------------------
// RSA operations
// ---------------------------------------------------------------------------

// rsaModExp computes value^65537 mod modulus (raw RSA encryption).
// Ported from Crypto.RSA2048Encrypt. All values are big-endian.
func rsaModExp(value, modulus []byte) []byte {
	// C# uses BigInteger(value.Reverse()) which is little-endian representation.
	// Go's big.Int.SetBytes treats input as big-endian.
	msg := new(big.Int).SetBytes(value)
	mod := new(big.Int).SetBytes(modulus)
	exp := big.NewInt(65537)

	result := new(big.Int).Exp(msg, exp, mod)

	// Pad/truncate to exactly 256 bytes, big-endian
	out := make([]byte, 256)
	resultBytes := result.Bytes()
	if len(resultBytes) > 256 {
		resultBytes = resultBytes[len(resultBytes)-256:]
	}
	copy(out[256-len(resultBytes):], resultBytes)
	return out
}

// rsaRawDecrypt performs raw RSA decryption (value^d mod n) using CRT.
// Ported from Crypto.DecryptEEKPfs / RSA2048Decrypt.
func rsaRawDecrypt(ciphertext []byte, ks *rsaKeyset) []byte {
	pub := rsa.PublicKey{
		N: new(big.Int).SetBytes(ks.Modulus),
		E: 65537,
	}
	priv := rsa.PrivateKey{
		PublicKey: pub,
		D:         new(big.Int).SetBytes(ks.PrivateExponent),
		Primes:    []*big.Int{new(big.Int).SetBytes(ks.Prime1), new(big.Int).SetBytes(ks.Prime2)},
	}
	priv.Precomputed.Dp = new(big.Int).SetBytes(ks.Exponent1)
	priv.Precomputed.Dq = new(big.Int).SetBytes(ks.Exponent2)
	priv.Precomputed.Qinv = new(big.Int).SetBytes(ks.Coefficient)

	// Raw RSA: m = c^d mod n
	c := new(big.Int).SetBytes(ciphertext)
	m := new(big.Int).Exp(c, priv.D, pub.N)
	result := m.Bytes()

	// Ensure 256 bytes, big-endian
	out := make([]byte, 256)
	if len(result) > 256 {
		result = result[len(result)-256:]
	}
	copy(out[256-len(result):], result)
	return out
}

// RSA2048EncryptKey encrypts a 32-byte hash with the given RSA modulus using
// Sony's custom padding scheme (Mersenne Twister PRNG).
// Ported from Crypto.RSA2048EncryptKey.
func RSA2048EncryptKey(modulus, hash []byte) []byte {
	// 1. Seed MT PRNG
	buf := make([]byte, 256+32)
	copy(buf[0:256], modulus)
	copy(buf[256:288], hash)
	finalHash := Sha256(Sha256(buf))

	// Convert hash to 8 uint32 (big-endian)
	var seeds [8]uint32
	for i := 0; i < 8; i++ {
		seeds[i] = binary.BigEndian.Uint32(finalHash[i*4 : i*4+4])
	}
	mt := newMersenneTwister(seeds[:])

	// 2. Build padded input: 00 02 [random non-zero bytes] 00 [hash 32 bytes]
	padded := make([]byte, 256)
	padded[0] = 0x00
	padded[1] = 0x02
	padded[223] = 0x00
	copy(padded[224:256], hash)

	// Fill bytes 2..222 with random non-zero bytes from MT PRNG
	shaSource := make([]byte, 48)
	for k := 2; k < 223; {
		for i := 0; i < 12; i++ {
			binary.BigEndian.PutUint32(shaSource[i*4:], mt.Uint32())
		}
		random := Sha256(shaSource)
		for _, r := range random {
			if k >= 223 {
				break
			}
			if r != 0 {
				padded[k] = r
				k++
			}
		}
	}

	// 3. Encrypt with raw RSA
	return rsaModExp(padded, modulus)
}

// RSA2048SignSha256 signs a SHA-256 hash with PKCS#1 v1.5 using the keyset.
func RSA2048SignSha256(hash []byte, ks *rsaKeyset) ([]byte, error) {
	pub := rsa.PublicKey{
		N: new(big.Int).SetBytes(ks.Modulus),
		E: 65537,
	}
	priv := rsa.PrivateKey{
		PublicKey: pub,
		D:         new(big.Int).SetBytes(ks.PrivateExponent),
		Primes:    []*big.Int{new(big.Int).SetBytes(ks.Prime1), new(big.Int).SetBytes(ks.Prime2)},
	}
	priv.Precomputed.Dp = new(big.Int).SetBytes(ks.Exponent1)
	priv.Precomputed.Dq = new(big.Int).SetBytes(ks.Exponent2)
	priv.Precomputed.Qinv = new(big.Int).SetBytes(ks.Coefficient)

	return rsa.SignPKCS1v15(rand.Reader, &priv, crypto.SHA256, hash)
}

// RSA2048VerifySha256 verifies a PKCS#1 v1.5 signature.
func RSA2048VerifySha256(hash, signature []byte, ks *rsaKeyset) error {
	pub := rsa.PublicKey{
		N: new(big.Int).SetBytes(ks.Modulus),
		E: 65537,
	}
	return rsa.VerifyPKCS1v15(&pub, crypto.SHA256, hash, signature)
}

// DecryptEEKPfs decrypts the EEKPFS value using RSA decryption with the fake keyset.
// Ported from Crypto.DecryptEEKPfs.
func DecryptEEKPfs(eekpfs []byte, ks *rsaKeyset) []byte {
	return rsaRawDecrypt(eekpfs, ks)
}

// ---------------------------------------------------------------------------
// AES-128-CBC
// ---------------------------------------------------------------------------

// AES128CBCEncrypt encrypts data with AES-128-CBC (no padding — input must be
// a multiple of 16 bytes). Ported from Crypto.AesCbcCfb128Encrypt.
func AES128CBCEncrypt(data, key, iv []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic("fpkg: AES cipher init: " + err.Error())
	}
	out := make([]byte, len(data))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(out, data)
	return out
}

// AES128CBCDecrypt decrypts data with AES-128-CBC.
func AES128CBCDecrypt(data, key, iv []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic("fpkg: AES cipher init: " + err.Error())
	}
	out := make([]byte, len(data))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(out, data)
	return out
}

// AES128CBCEncryptPad encrypts data with AES-128-CBC, padding to block size
// with zeros if necessary.
func AES128CBCEncryptPad(data, key, iv []byte) []byte {
	// Pad to multiple of 16
	padded := make([]byte, ((len(data)+15)/16)*16)
	copy(padded, data)
	return AES128CBCEncrypt(padded, key, iv)
}

// ---------------------------------------------------------------------------
// AES-128-XTS
// ---------------------------------------------------------------------------

// AES128XTSEncrypt encrypts data with AES-128-XTS.
// sectorSize is the size of each sector (typically 0x1000).
// startSector is the first sector to encrypt (typically BlockSize/sectorSize = 16).
// Sectors before startSector are left in plaintext.
// Ported from XtsBlockTransform.
func AES128XTSEncrypt(data, dataKey, tweakKey []byte, sectorSize, startSector int) []byte {
	dataCipher, err := aes.NewCipher(dataKey)
	if err != nil {
		panic("fpkg: AES data key: " + err.Error())
	}
	tweakCipher, err := aes.NewCipher(tweakKey)
	if err != nil {
		panic("fpkg: AES tweak key: " + err.Error())
	}

	out := make([]byte, len(data))
	copy(out, data)

	for sectorNum := 0; sectorNum*sectorSize < len(out); sectorNum++ {
		// Skip sectors before startSector (leave plaintext)
		if sectorNum < startSector {
			continue
		}
		start := sectorNum * sectorSize
		end := start + sectorSize
		if end > len(out) {
			end = len(out)
		}
		sector := out[start:end]
		xtsEncryptSector(sector, uint64(sectorNum), dataCipher, tweakCipher)
	}

	return out
}

// xtsEncryptSector encrypts a single sector with XEX mode.
func xtsEncryptSector(sector []byte, sectorNum uint64, dataCipher, tweakCipher cipher.Block) {
	// Build tweak from sector number
	tweak := make([]byte, 16)
	binary.LittleEndian.PutUint64(tweak[0:8], sectorNum)

	encryptedTweak := make([]byte, 16)
	tweakCipher.Encrypt(encryptedTweak, tweak)

	for offset := 0; offset < len(sector); offset += 16 {
		// XOR plaintext with encrypted tweak
		for x := 0; x < 16; x++ {
			sector[offset+x] ^= encryptedTweak[x]
		}
		// AES-ECB encrypt
		dataCipher.Encrypt(sector[offset:offset+16], sector[offset:offset+16])
		// XOR ciphertext with encrypted tweak
		for x := 0; x < 16; x++ {
			sector[offset+x] ^= encryptedTweak[x]
		}
		// GF-multiply tweak by alpha (x^128 + x^7 + x^2 + x + 1 in GF(2^128))
		var feedback uint8
		for k := 0; k < 16; k++ {
			tmp := encryptedTweak[k]
			encryptedTweak[k] = (encryptedTweak[k] << 1) | feedback
			feedback = tmp >> 7
		}
		if feedback != 0 {
			encryptedTweak[0] ^= 0x87
		}
	}
}

// ---------------------------------------------------------------------------
// Keystone
// ---------------------------------------------------------------------------

// CreateKeystone builds a keystone blob from the given passcode.
// Ported from Crypto.CreateKeystone.
func CreateKeystone(passcode string) []byte {
	keystoneHeader := hexDecode("6b657973746f6e65020001000000000000000000000000000000000000000000")
	fingerprint := HmacSha256(KeystoneHMACKey, []byte(passcode))
	final := HmacSha256(KeystoneMACData, append(keystoneHeader, fingerprint...))
	return append(append(keystoneHeader, fingerprint...), final...)
}

// ---------------------------------------------------------------------------
// Mersenne Twister PRNG
// ---------------------------------------------------------------------------

const mtN = 624
const mtM = 397
const mtDefaultSeed uint32 = 0x12BD6AA
const matrixA uint32 = 0x9908b0df
const upperMask uint32 = 0x80000000
const lowerMask uint32 = 0x7fffffff
const constant1 uint32 = 0x6C078965
const constant2 uint32 = 0x19660D
const constant3 uint32 = 0x5D588B65
const constant4 uint32 = 0x9d2c5680
const constant5 uint32 = 0xefc60000

// mersenneTwister is a Mersenne Twister MT19937 PRNG.
// Ported from LibOrbisPkg/Util/MersenneTwister.cs.
type mersenneTwister struct {
	mt  [mtN]uint32
	mti int
}

func newMersenneTwister(seeds []uint32) *mersenneTwister {
	mt := &mersenneTwister{}

	// Basic init
	mt.mt[0] = mtDefaultSeed
	for i := 1; i < mtN; i++ {
		mt.mt[i] = uint32(i) + constant1*(mt.mt[i-1]^(mt.mt[i-1]>>30))
	}
	mt.mti = mtN

	// Seed with array
	stateIdx := 1
	seedIdx := 0
	for length := max(mtN, len(seeds)); length > 0; length-- {
		mt.mt[stateIdx] = (mt.mt[stateIdx] ^ ((mt.mt[stateIdx-1] ^ (mt.mt[stateIdx-1] >> 30)) * constant2)) + seeds[seedIdx] + uint32(seedIdx)
		stateIdx++
		seedIdx++
		if stateIdx >= mtN {
			mt.mt[0] = mt.mt[mtN-1]
			stateIdx = 1
		}
		if seedIdx >= len(seeds) {
			seedIdx = 0
		}
	}
	for length := 0; length < mtN-1; length++ {
		mt.mt[stateIdx] = (mt.mt[stateIdx] ^ ((mt.mt[stateIdx-1] ^ (mt.mt[stateIdx-1] >> 30)) * constant3)) - uint32(stateIdx)
		stateIdx++
		if stateIdx >= mtN {
			mt.mt[0] = mt.mt[mtN-1]
			stateIdx = 1
		}
	}
	mt.mt[0] = 1 << 31 // MSB is 1; assuring non-zero initial array

	return mt
}

func (mt *mersenneTwister) Uint32() uint32 {
	mag01 := [2]uint32{0, matrixA}

	if mt.mti >= mtN {
		// Generate N words at once
		var kk int
		for kk = 0; kk < mtN-mtM; kk++ {
			y := (mt.mt[kk] & upperMask) | (mt.mt[kk+1] & lowerMask)
			mt.mt[kk] = mt.mt[kk+mtM] ^ ((y >> 1) & lowerMask) ^ mag01[y&1]
		}
		for ; kk < mtN-1; kk++ {
			y := (mt.mt[kk] & upperMask) | (mt.mt[kk+1] & lowerMask)
			mt.mt[kk] = mt.mt[kk+mtM-mtN] ^ ((y >> 1) & lowerMask) ^ mag01[y&1]
		}
		y := (mt.mt[mtN-1] & upperMask) | (mt.mt[0] & lowerMask)
		mt.mt[mtN-1] = mt.mt[mtM-1] ^ ((y >> 1) & lowerMask) ^ mag01[y&1]
		mt.mti = 0
	}

	y := mt.mt[mt.mti]
	mt.mti++

	// Tempering
	y ^= (y >> 11) & 0x001FFFFF
	y ^= (y << 7) & constant4
	y ^= (y << 15) & constant5
	y ^= (y >> 18) & 0x00003FFF

	return y
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

func hexDecode(s string) []byte {
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		var hi, lo byte
		switch {
		case s[i] >= '0' && s[i] <= '9':
			hi = s[i] - '0'
		case s[i] >= 'a' && s[i] <= 'f':
			hi = s[i] - 'a' + 10
		case s[i] >= 'A' && s[i] <= 'F':
			hi = s[i] - 'A' + 10
		}
		switch {
		case s[i+1] >= '0' && s[i+1] <= '9':
			lo = s[i+1] - '0'
		case s[i+1] >= 'a' && s[i+1] <= 'f':
			lo = s[i+1] - 'a' + 10
		case s[i+1] >= 'A' && s[i+1] <= 'F':
			lo = s[i+1] - 'A' + 10
		}
		b[i/2] = (hi << 4) | lo
	}
	return b
}


