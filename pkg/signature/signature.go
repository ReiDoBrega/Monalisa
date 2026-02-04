package signature

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	// Chave mestra do sistema (ALTERE ESTA CHAVE PARA SUA PRÓPRIA CHAVE SECRETA)
	masterKey = "7282cd71ff7393085cc702d0cea433f90420eb04e26857620e11d3cd951dab86"

	// Chave para ofuscar timestamps internos
	timeObfuscationKey = "OId6KowkmNaXo1drlZ933MkA"
)

type Signature struct {
	DeviceID         string
	CreationTime     time.Time // Timestamp de criação (ofuscado)
	ExpiryTime       time.Time // Timestamp de expiração (ofuscado)
	MaxValidDuration int64     // Duração máxima em segundos (ofuscado)
	HMAC             []byte
}

// GenerateDeviceID gera um ID único para o device
func GenerateDeviceID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// obfuscateInt64 ofusca um valor int64 usando XOR
func obfuscateInt64(value int64) int64 {
	key := int64(0)
	for i, c := range timeObfuscationKey {
		key ^= int64(c) << (i % 8)
	}
	return value ^ key
}

// deobfuscateInt64 recupera o valor original
func deobfuscateInt64(obfuscated int64) int64 {
	return obfuscateInt64(obfuscated) // XOR é reversível
}

// Sign cria uma assinatura para o device com data de expiração
func Sign(deviceID string, validDays int) (*Signature, error) {
	now := time.Now()
	expiryTime := now.AddDate(0, 0, validDays)
	maxDuration := int64(validDays * 24 * 60 * 60) // em segundos

	// Cria o payload: deviceID + creation + expiry + duration
	message := fmt.Sprintf("%s:%d:%d:%d",
		deviceID,
		now.Unix(),
		expiryTime.Unix(),
		maxDuration)

	// Gera HMAC-SHA256
	h := hmac.New(sha256.New, []byte(masterKey))
	h.Write([]byte(message))
	signature := h.Sum(nil)

	return &Signature{
		DeviceID:         deviceID,
		CreationTime:     now,
		ExpiryTime:       expiryTime,
		MaxValidDuration: maxDuration,
		HMAC:             signature,
	}, nil
}

// Verify verifica se a assinatura é válida e detecta adulteração de relógio
func Verify(deviceID string, creationTimestamp, expiryTimestamp, maxDuration int64, signatureBytes []byte) error {
	creationTime := time.Unix(creationTimestamp, 0)
	expiryTime := time.Unix(expiryTimestamp, 0)
	now := time.Now()

	// PROTEÇÃO 1: Verifica se o relógio está ANTES da data de criação
	// (impossível usar o device antes de ser criado)
	if now.Before(creationTime) {
		daysBackwards := int(creationTime.Sub(now).Hours() / 24)
		return fmt.Errorf("clock tampering detected: system time is %d day(s) before device creation (created: %s, current: %s)",
			daysBackwards,
			creationTime.Format("2006-01-02 15:04:05"),
			now.Format("2006-01-02 15:04:05"))
	}

	// PROTEÇÃO 2: Verifica se passou do tempo de expiração
	if now.After(expiryTime) {
		daysExpired := int(now.Sub(expiryTime).Hours() / 24)
		return fmt.Errorf("device license expired %d day(s) ago (expired: %s)",
			daysExpired,
			expiryTime.Format("2006-01-02"))
	}

	// PROTEÇÃO 3: Verifica se o tempo decorrido desde a criação excede a duração máxima
	// Isso impede que alguém volte o relógio para uma data entre criação e expiração
	elapsedTime := now.Unix() - creationTimestamp
	if elapsedTime > maxDuration {
		return fmt.Errorf("maximum license duration exceeded (elapsed: %d days, max: %d days)",
			elapsedTime/(24*60*60),
			maxDuration/(24*60*60))
	}

	// PROTEÇÃO 4: Valida integridade da assinatura HMAC
	message := fmt.Sprintf("%s:%d:%d:%d", deviceID, creationTimestamp, expiryTimestamp, maxDuration)
	h := hmac.New(sha256.New, []byte(masterKey))
	h.Write([]byte(message))
	expectedHMAC := h.Sum(nil)

	if !hmac.Equal(signatureBytes, expectedHMAC) {
		return fmt.Errorf("invalid device signature: file may be corrupted or tampered")
	}

	return nil
}

// Encode converte a assinatura para bytes (com ofuscação)
func (s *Signature) Encode() []byte {
	// Formato: [deviceID (32)] + [creation_obf (8)] + [expiry_obf (8)] + [duration_obf (8)] + [HMAC (32)]
	// Total: 88 bytes
	result := make([]byte, 88)

	// Device ID (32 bytes hex)
	copy(result[0:32], s.DeviceID)

	// Creation timestamp ofuscado (8 bytes)
	obfCreation := obfuscateInt64(s.CreationTime.Unix())
	binary.LittleEndian.PutUint64(result[32:40], uint64(obfCreation))

	// Expiry timestamp ofuscado (8 bytes)
	obfExpiry := obfuscateInt64(s.ExpiryTime.Unix())
	binary.LittleEndian.PutUint64(result[40:48], uint64(obfExpiry))

	// Max duration ofuscado (8 bytes)
	obfDuration := obfuscateInt64(s.MaxValidDuration)
	binary.LittleEndian.PutUint64(result[48:56], uint64(obfDuration))

	// HMAC (32 bytes)
	copy(result[56:88], s.HMAC)

	return result
}

// Decode extrai a assinatura dos bytes (com deofuscação)
func Decode(data []byte) (*Signature, error) {
	if len(data) != 88 {
		return nil, fmt.Errorf("invalid signature length: expected 88 bytes, got %d", len(data))
	}

	deviceID := string(data[0:32])

	// Deofusca os timestamps
	obfCreation := int64(binary.LittleEndian.Uint64(data[32:40]))
	creationTimestamp := deobfuscateInt64(obfCreation)

	obfExpiry := int64(binary.LittleEndian.Uint64(data[40:48]))
	expiryTimestamp := deobfuscateInt64(obfExpiry)

	obfDuration := int64(binary.LittleEndian.Uint64(data[48:56]))
	maxDuration := deobfuscateInt64(obfDuration)

	hmacBytes := make([]byte, 32)
	copy(hmacBytes, data[56:88])

	return &Signature{
		DeviceID:         deviceID,
		CreationTime:     time.Unix(creationTimestamp, 0),
		ExpiryTime:       time.Unix(expiryTimestamp, 0),
		MaxValidDuration: maxDuration,
		HMAC:             hmacBytes,
	}, nil
}

// GetCreationTimestamp retorna o timestamp de criação (para validação)
func (s *Signature) GetCreationTimestamp() int64 {
	return s.CreationTime.Unix()
}

// GetExpiryTimestamp retorna o timestamp de expiração (para validação)
func (s *Signature) GetExpiryTimestamp() int64 {
	return s.ExpiryTime.Unix()
}

// GetMaxDuration retorna a duração máxima (para validação)
func (s *Signature) GetMaxDuration() int64 {
	return s.MaxValidDuration
}
