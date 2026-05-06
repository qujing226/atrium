package secure

import (
	"crypto/hmac"
	"golang.org/x/crypto/sha3"
	"errors"
	"hash"
)

// KDF-Chain 常量，用于区分消息密钥和下一跳链密钥的生成
var (
	inputMsgKey  = []byte{0x01} // 用于生成 Message Key
	inputNextKey = []byte{0x02} // 用于生成 Next Chain Key
)

// ChainKey 实现了基于 HMAC 的对称密钥棘轮 (Symmetric Ratchet)
// 对应论文中的 "Key Evolution" 或 "Hash Chain"
type ChainKey struct {
	key []byte // 当前的链密钥 (32 bytes)
}

// NewChainKey 使用初始共享密钥 (如 Kyber SS) 初始化棘轮
func NewChainKey(initialSecret []byte) *ChainKey {
	// 建议 initialSecret 长度至少 32 字节
	k := make([]byte, len(initialSecret))
	copy(k, initialSecret)
	return &ChainKey{key: k}
}

// Ratchet 执行一步棘轮演化
// 返回:
// 1. messageKey: 本次通信专用的加密密钥 (前向安全)
// 2. error: 如果内部状态异常
// 副作用: 内部的 ChainKey 会更新为下一跳状态
func (c *ChainKey) Ratchet() (messageKey []byte, err error) {
	if len(c.key) == 0 {
		return nil, errors.New("chain key is empty")
	}

	// 1. 派生 Message Key (用于加密数据)
	// MK = HMAC-SHA256(CK, 0x01)
	mkMac := hmac.New(sha3.New384, c.key)
	mkMac.Write(inputMsgKey)
	messageKey = mkMac.Sum(nil)

	// 2. 派生 Next Chain Key (用于下一次棘轮)
	// CK_next = HMAC-SHA256(CK, 0x02)
	ckMac := hmac.New(sha3.New384, c.key)
	ckMac.Write(inputNextKey)
	nextChainKey := ckMac.Sum(nil)

	// 3. 更新内部状态 (Destroy old key -> Forward Secrecy)
	c.key = nextChainKey

	return messageKey, nil
}

// CurrentState 用于调试或持久化 (生产环境慎用)
func (c *ChainKey) CurrentState() []byte {
	// 返回副本
	out := make([]byte, len(c.key))
	copy(out, c.key)
	return out
}

// SimpleKDF 是一个通用的密钥派生函数，用于将 Kyber 的随机种子扩展为初始 ChainKey
// 使用 HKDF-like 结构: HMAC(salt, info)
func SimpleKDF(secret, salt, info []byte) []byte {
	h := func() hash.Hash { return sha3.New384() }
	
	// Extract (if salt is provided)
	prk := secret
	if len(salt) > 0 {
		mac := hmac.New(h, salt)
		mac.Write(secret)
		prk = mac.Sum(nil)
	}

	// Expand
	mac := hmac.New(h, prk)
	mac.Write(info)
	// Counter (0x01)
	mac.Write([]byte{0x01}) 
	return mac.Sum(nil)
}
