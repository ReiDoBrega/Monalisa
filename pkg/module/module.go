package module

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"monalisa/pkg/exceptions"
	"monalisa/pkg/logger"
	"monalisa/pkg/signature"

	"github.com/bytecodealliance/wasmtime-go/v25"
)

const (
	Magic              = "MLD\x00"
	FlagCompressedMeta = 0x0001
	FlagCompressedWASM = 0x0002
	FlagCompressedJS   = 0x0004
	FlagSigned         = 0x0008
)

type Module struct {
	wasmBytes []byte
	metadata  map[string]interface{}
	jsCode    []byte
	engine    *wasmtime.Engine
	module    *wasmtime.Module
	signature *signature.Signature
}

func Load(mldPath string) (*Module, error) {
	file, err := os.Open(mldPath)
	if err != nil {
		return nil, exceptions.NewModuleError("Module file not found: %s", mldPath)
	}
	defer file.Close()

	header := make([]byte, 32)
	if _, err := io.ReadFull(file, header); err != nil {
		return nil, exceptions.NewModuleError("Invalid MLD file: header too short")
	}

	if string(header[:4]) != Magic {
		return nil, exceptions.NewModuleError("Invalid MLD file: bad magic number")
	}

	flags := binary.LittleEndian.Uint16(header[6:8])
	metadataSize := binary.LittleEndian.Uint32(header[8:12])
	wasmSize := binary.LittleEndian.Uint32(header[12:16])
	jsSize := binary.LittleEndian.Uint32(header[16:20])
	checksum := binary.LittleEndian.Uint32(header[20:24])
	signatureSize := binary.LittleEndian.Uint32(header[24:28])

	payload, err := io.ReadAll(file)
	if err != nil {
		return nil, exceptions.NewModuleError("Failed to read payload: %v", err)
	}

	if crc32.ChecksumIEEE(payload) != checksum {
		return nil, exceptions.NewModuleError("Checksum mismatch: file corrupted")
	}

	offset := uint32(0)

	// Lê e verifica assinatura se presente
	var sig *signature.Signature
	if flags&FlagSigned != 0 {
		if signatureSize != 88 {
			return nil, exceptions.NewModuleError("Invalid signature size: expected 88 bytes, got %d", signatureSize)
		}

		sigBytes := payload[offset : offset+signatureSize]
		offset += signatureSize

		sig, err = signature.Decode(sigBytes)
		if err != nil {
			return nil, exceptions.NewModuleError("Failed to decode signature: %v", err)
		}

		// Verifica a assinatura com proteção anti-adulteração de relógio
		if err := signature.Verify(
			sig.DeviceID,
			sig.GetCreationTimestamp(),
			sig.GetExpiryTimestamp(),
			sig.GetMaxDuration(),
			sig.HMAC,
		); err != nil {
			return nil, exceptions.NewModuleError("License verification failed: %v", err)
		}
	}

	metadataBytes := payload[offset : offset+metadataSize]
	offset += metadataSize

	if flags&FlagCompressedMeta != 0 {
		metadataBytes, err = decompress(metadataBytes)
		if err != nil {
			return nil, exceptions.NewModuleError("Failed to decompress metadata: %v", err)
		}
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, exceptions.NewModuleError("Invalid metadata JSON: %v", err)
	}

	wasmBytes := payload[offset : offset+wasmSize]
	offset += wasmSize

	if flags&FlagCompressedWASM != 0 {
		wasmBytes, err = decompress(wasmBytes)
		if err != nil {
			return nil, exceptions.NewModuleError("Failed to decompress WASM: %v", err)
		}
	}

	var jsCode []byte
	if jsSize > 0 {
		jsBytes := payload[offset : offset+jsSize]
		if flags&FlagCompressedJS != 0 {
			jsBytes, err = decompress(jsBytes)
			if err != nil {
				return nil, exceptions.NewModuleError("Failed to decompress JS: %v", err)
			}
		}
		jsCode = jsBytes
	}

	engine := wasmtime.NewEngine()
	wasmModule, err := wasmtime.NewModule(engine, wasmBytes)
	if err != nil {
		return nil, exceptions.NewModuleError("Failed to load WASM module: %v", err)
	}

	return &Module{
		wasmBytes: wasmBytes,
		metadata:  metadata,
		jsCode:    jsCode,
		engine:    engine,
		module:    wasmModule,
		signature: sig,
	}, nil
}

func (m *Module) CreateStore() *wasmtime.Store {
	return wasmtime.NewStore(m.engine)
}

func (m *Module) WASMModule() *wasmtime.Module {
	return m.module
}

func (m *Module) Signature() *signature.Signature {
	return m.signature
}

func decompress(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

type Builder struct {
	metadata  map[string]interface{}
	wasmData  []byte
	jsData    []byte
	compress  bool
	validDays int
}

func NewBuilder() *Builder {
	return &Builder{
		metadata:  make(map[string]interface{}),
		compress:  true,
		validDays: 0, // 0 = sem assinatura
	}
}

func (b *Builder) WithMetadata(version, name, description string) *Builder {
	b.metadata = map[string]interface{}{
		"version":        version,
		"name":           name,
		"description":    description,
		"format":         "binary-mld",
		"self_contained": true,
	}
	return b
}

func (b *Builder) WithWAT(watPath string) error {
	tmpFile, err := os.CreateTemp("", "*.wasm")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	cmd := exec.Command("wat2wasm", watPath, "-o", tmpPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wat2wasm failed (install WABT toolkit): %v", err)
	}

	b.wasmData, err = os.ReadFile(tmpPath)
	return err
}

func (b *Builder) WithWASM(wasmPath string) error {
	data, err := os.ReadFile(wasmPath)
	if err != nil {
		return err
	}
	b.wasmData = data
	logger.Debug("  WASM loaded: %d bytes", len(data))
	return nil
}

func (b *Builder) WithJS(jsPath string) error {
	data, err := os.ReadFile(jsPath)
	if err != nil {
		return err
	}
	b.jsData = data
	logger.Debug("  JS loaded: %d bytes", len(data))
	return nil
}

func (b *Builder) WithCompression(enable bool) *Builder {
	b.compress = enable
	return b
}

func (b *Builder) WithLicense(validDays int) *Builder {
	b.validDays = validDays
	return b
}

func (b *Builder) Build(outputPath string) error {
	if b.wasmData == nil {
		return fmt.Errorf("no WASM data provided")
	}

	metadataJSON, _ := json.MarshalIndent(b.metadata, "", "  ")

	flags := uint16(0)
	var metaComp, wasmComp, jsComp []byte

	if b.compress {
		metaComp = compress(metadataJSON)
		flags |= FlagCompressedMeta
		logger.Debug("  Metadata: %d → %d bytes (%.1f%%)",
			len(metadataJSON), len(metaComp), 100.0*float64(len(metaComp))/float64(len(metadataJSON)))
	} else {
		metaComp = metadataJSON
	}

	if b.compress {
		wasmComp = compress(b.wasmData)
		flags |= FlagCompressedWASM
		logger.Debug("  WASM: %d → %d bytes (%.1f%%)",
			len(b.wasmData), len(wasmComp), 100.0*float64(len(wasmComp))/float64(len(b.wasmData)))
	} else {
		wasmComp = b.wasmData
	}

	if b.jsData != nil {
		if b.compress {
			jsComp = compress(b.jsData)
			flags |= FlagCompressedJS
			logger.Debug("  JS: %d → %d bytes (%.1f%%)",
				len(b.jsData), len(jsComp), 100.0*float64(len(jsComp))/float64(len(b.jsData)))
		} else {
			jsComp = b.jsData
		}
	}

	// Gera assinatura se validDays > 0
	var sigBytes []byte
	if b.validDays > 0 {
		deviceID, err := signature.GenerateDeviceID()
		if err != nil {
			return fmt.Errorf("failed to generate device ID: %v", err)
		}

		sig, err := signature.Sign(deviceID, b.validDays)
		if err != nil {
			return fmt.Errorf("failed to sign device: %v", err)
		}

		sigBytes = sig.Encode()
		flags |= FlagSigned

		logger.Debug("  License:")
		logger.Debug("    Device ID: %s", deviceID)
		logger.Debug("    Created: %s", sig.CreationTime.Format("2006-01-02 15:04:05"))
		logger.Debug("    Expires: %s (%d days)", sig.ExpiryTime.Format("2006-01-02 15:04:05"), b.validDays)
		logger.Debug("    Max Duration: %d seconds", sig.MaxValidDuration)
	}

	payload := bytes.NewBuffer(nil)
	if sigBytes != nil {
		payload.Write(sigBytes)
	}
	payload.Write(metaComp)
	payload.Write(wasmComp)
	if jsComp != nil {
		payload.Write(jsComp)
	}

	checksum := crc32.ChecksumIEEE(payload.Bytes())

	header := bytes.NewBuffer(nil)
	header.WriteString(Magic)
	header.WriteByte(1) // major
	header.WriteByte(0) // minor
	binary.Write(header, binary.LittleEndian, flags)
	binary.Write(header, binary.LittleEndian, uint32(len(metaComp)))
	binary.Write(header, binary.LittleEndian, uint32(len(wasmComp)))
	binary.Write(header, binary.LittleEndian, uint32(len(jsComp)))
	binary.Write(header, binary.LittleEndian, checksum)
	binary.Write(header, binary.LittleEndian, uint32(len(sigBytes)))
	header.Write(make([]byte, 4)) // reserved

	os.MkdirAll(filepath.Dir(outputPath), 0755)
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	file.Write(header.Bytes())
	file.Write(payload.Bytes())

	logger.Debug("  Total: %d bytes", len(header.Bytes())+len(payload.Bytes()))
	logger.Debug("  Checksum: %08x", checksum)
	return nil
}

func (b *Builder) Verify(modulePath string) error {
	file, err := os.Open(modulePath)
	if err != nil {
		return err
	}
	defer file.Close()

	header := make([]byte, 32)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("header too short")
	}

	if string(header[:4]) != Magic {
		return fmt.Errorf("not a binary MLD file")
	}

	checksum := binary.LittleEndian.Uint32(header[20:24])
	payload, _ := io.ReadAll(file)

	if crc32.ChecksumIEEE(payload) != checksum {
		return fmt.Errorf("checksum mismatch")
	}

	return nil
}

func compress(data []byte) []byte {
	var buf bytes.Buffer
	w, _ := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	w.Write(data)
	w.Close()
	return buf.Bytes()
}
