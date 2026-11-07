// Package passwordcrypto 提供浏览器登录密码的传输加密基础能力：服务端持
// 有 RSA-2048 私钥，公钥经专用端点下发，前端以 RSA-OAEP/SHA-256 加密口令
// 后提交。该机制是纵深防御（opt-in 开关控制），不能替代 HTTPS。
package passwordcrypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"sync"
)

const passwordEncryptionKeyBits = 2048

// ErrPasswordEncryptionInvalid 所有畸形输入共用同一错误，调用方对未认证
// 客户端只返回单一提示，不泄露密码学细节。
var ErrPasswordEncryptionInvalid = errors.New("password encryption payload is invalid")

var passwordEncryptionState struct {
	sync.RWMutex
	privateKey *rsa.PrivateKey
	publicKey  string
	keyID      string
}

// GeneratePrivateKeyPEM 生成登录加密用 RSA-2048 PKCS#8 私钥 PEM，持久化由调用方负责。
func GeneratePrivateKeyPEM() (string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, passwordEncryptionKeyBits)
	if err != nil {
		return "", fmt.Errorf("generate password encryption key: %w", err)
	}
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", fmt.Errorf("marshal password encryption key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyDER,
	})), nil
}

// LoadPrivateKeyPEM 校验持久化私钥（PKCS#8、RSA、不低于 2048 位）后替换
// 内存中的活跃密钥，并派生公钥 PEM 与密钥标识（SPKI DER SHA-256 前 16 字
// 节 hex）。
func LoadPrivateKeyPEM(privateKeyPEM string) error {
	block, rest := pem.Decode([]byte(privateKeyPEM))
	if block == nil || block.Type != "PRIVATE KEY" || strings.TrimSpace(string(rest)) != "" {
		return errors.New("password encryption key is not valid PKCS#8 PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse password encryption key: %w", err)
	}
	privateKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return errors.New("password encryption key is not RSA")
	}
	if privateKey.N == nil || privateKey.N.BitLen() < passwordEncryptionKeyBits {
		return fmt.Errorf("password encryption key must be at least %d bits", passwordEncryptionKeyBits)
	}
	if err := privateKey.Validate(); err != nil {
		return fmt.Errorf("validate password encryption key: %w", err)
	}
	privateKey.Precompute()

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("marshal password encryption public key: %w", err)
	}
	publicKeyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	}))
	keyDigest := sha256.Sum256(publicKeyDER)
	keyID := hex.EncodeToString(keyDigest[:16])

	passwordEncryptionState.Lock()
	defer passwordEncryptionState.Unlock()
	passwordEncryptionState.privateKey = privateKey
	passwordEncryptionState.publicKey = publicKeyPEM
	passwordEncryptionState.keyID = keyID
	return nil
}

// PasswordEncryptionPublicKey 返回活跃密钥标识与 SPKI PEM 公钥，供专用端
// 点下发给浏览器客户端。
func PasswordEncryptionPublicKey() (keyID string, publicKeyPEM string) {
	passwordEncryptionState.RLock()
	defer passwordEncryptionState.RUnlock()
	return passwordEncryptionState.keyID, passwordEncryptionState.publicKey
}

// DecryptPassword 解密前端提交的 base64 RSA-OAEP/SHA-256 口令密文；密钥标
// 识不匹配、编码或长度非法、解密失败一律返回同一错误。
func DecryptPassword(ciphertextBase64 string, keyID string) (string, error) {
	passwordEncryptionState.RLock()
	privateKey := passwordEncryptionState.privateKey
	activeKeyID := passwordEncryptionState.keyID
	passwordEncryptionState.RUnlock()
	if privateKey == nil || keyID == "" || keyID != activeKeyID {
		return "", ErrPasswordEncryptionInvalid
	}
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil || len(ciphertext) != privateKey.Size() {
		return "", ErrPasswordEncryptionInvalid
	}
	plaintext, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, ciphertext, nil)
	if err != nil || len(plaintext) == 0 {
		return "", ErrPasswordEncryptionInvalid
	}
	return string(plaintext), nil
}
