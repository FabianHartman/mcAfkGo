package CFB8

import (
	"crypto/cipher"
	"crypto/subtle"
	"unsafe"
)

type CFB8 struct {
	c         cipher.Block
	blockSize int
	ivPos     int
	iv        []byte
	de        bool
}

func NewCFB8Decrypt(c cipher.Block, iv []byte) *CFB8 {
	return newCFB8(c, iv, true)
}

func NewCFB8Encrypt(c cipher.Block, iv []byte) *CFB8 {
	return newCFB8(c, iv, false)
}

func newCFB8(c cipher.Block, iv []byte, de bool) *CFB8 {
	cp := make([]byte, len(iv)*3)
	copy(cp, iv)
	return &CFB8{
		c:         c,
		blockSize: c.BlockSize(),
		iv:        cp,
		de:        de,
	}
}

func (cf *CFB8) XORKeyStream(dst, src []byte) {
	if len(src) == 0 {
		return
	}

	if len(dst) < len(src) {
		panic("cfb8: output smaller than input")
	}

	if len(src) > cf.blockSize<<1 &&
		(uintptr(unsafe.Pointer(&dst[0]))+uintptr(cf.blockSize) <= uintptr(unsafe.Pointer(&src[0])) ||
			uintptr(unsafe.Pointer(&src[0]))+uintptr(len(src)) <= uintptr(unsafe.Pointer(&dst[0]))) {
		cf.xorKeyStream(dst, src[:cf.blockSize])
		var ciphertext []byte
		if cf.de {
			ciphertext = src
		} else {
			ciphertext = dst
		}

		dst = dst[cf.blockSize:]
		src = src[cf.blockSize:]
		iv := cf.iv
		_ = iv[0]
		var (
			i   int
			val byte
		)

		dst = dst[:len(src)]
		if cf.de &&
			uintptr(unsafe.Pointer(&dst[0])) <= uintptr(unsafe.Pointer(&src[len(src)-1])) &&
			uintptr(unsafe.Pointer(&src[0])) <= uintptr(unsafe.Pointer(&dst[len(dst)-1])) {
			for i = 0; i < len(src)-cf.blockSize; i += 1 {
				cf.c.Encrypt(dst[i:], ciphertext[i:])
			}

			subtle.XORBytes(dst, src[:i], dst)
			for ; i < len(src); i += 1 {
				cf.c.Encrypt(iv, ciphertext[i:])
				dst[i] = src[i] ^ iv[0]
			}
		} else {
			_ = ciphertext[len(src)]
			for i, val = range src {
				cf.c.Encrypt(iv, ciphertext[i:])
				dst[i] = val ^ iv[0]
			}

			i += 1
		}

		copy(iv, ciphertext[i:i+cf.blockSize])
		cf.ivPos = 0

		return
	}

	cf.xorKeyStream(dst, src)
}

func (cf *CFB8) xorKeyStream(dst, src []byte) {
	dst = dst[:len(src)]
	for i, val := range src {
		posPlusBlockSize := cf.ivPos + cf.blockSize
		tempPos := posPlusBlockSize & (cf.blockSize<<1 - 1)
		cf.c.Encrypt(cf.iv[tempPos:], cf.iv[cf.ivPos:])
		val ^= cf.iv[tempPos]

		if cf.ivPos == cf.blockSize<<1 {
			copy(cf.iv, cf.iv[cf.ivPos+1:])
			if cf.de {
				cf.iv[cf.blockSize-1] = src[i]
			} else {
				cf.iv[cf.blockSize-1] = val
			}

			cf.ivPos = 0
		} else {
			if cf.de {
				cf.iv[posPlusBlockSize] = src[i]
			} else {
				cf.iv[posPlusBlockSize] = val
			}

			cf.ivPos += 1
		}

		dst[i] = val
	}
}
