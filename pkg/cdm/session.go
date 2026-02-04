package cdm

import (
	"encoding/base64"
	"encoding/hex"
	"regexp"

	"monalisa/pkg/exceptions"
	"monalisa/pkg/license"
	"monalisa/pkg/module"
	"monalisa/pkg/types"

	"github.com/bytecodealliance/wasmtime-go/v25"
	"github.com/google/uuid"
)

const (
	DynamicBase      = 6065008
	DynamicTopPtr    = 821968
	LicenseKeyOffset = 0x5C8C0C
	LicenseKeyLength = 16
)

type Session struct {
	sessionID string
	module    *module.Module
	store     *wasmtime.Store
	instance  *wasmtime.Instance
	memory    *wasmtime.Memory
	exports   map[string]*wasmtime.Func
	ctx       int32
	keys      []*types.Key
}

func NewSession(sid string, mod *module.Module, st *wasmtime.Store) *Session {
	return &Session{
		sessionID: sid,
		module:    mod,
		store:     st,
		exports:   make(map[string]*wasmtime.Func),
		keys:      make([]*types.Key, 0),
	}
}

func (s *Session) Initialize() error {
	memType := wasmtime.NewMemoryType(256, true, 256)
	memory, err := wasmtime.NewMemory(s.store, memType)
	if err != nil {
		return exceptions.NewSessionError("Failed to create memory: %v", err)
	}
	s.memory = memory

	s.writeI32(DynamicTopPtr, DynamicBase)

	imports, err := s.buildImports()
	if err != nil {
		return exceptions.NewSessionError("Failed to build imports: %v", err)
	}

	inst, err := wasmtime.NewInstance(s.store, s.module.WASMModule(), imports)
	if err != nil {
		return exceptions.NewSessionError("Failed to create instance: %v", err)
	}
	s.instance = inst

	s.exports["___wasm_call_ctors"] = s.getExport("s").Func()
	s.exports["_monalisa_context_alloc"] = s.getExport("D").Func()
	s.exports["monalisa_set_license"] = s.getExport("F").Func()
	s.exports["stackAlloc"] = s.getExport("N").Func()
	s.exports["stackSave"] = s.getExport("L").Func()
	s.exports["stackRestore"] = s.getExport("M").Func()

	s.exports["___wasm_call_ctors"].Call(s.store)
	result, _ := s.exports["_monalisa_context_alloc"].Call(s.store)
	s.ctx = result.(int32)

	return nil
}

func (s *Session) ParseLicense(lic *license.License) error {
	licStr := lic.Base64()

	ret, err := s.ccallInt("monalisa_set_license", s.ctx, licStr, len(licStr), "0")
	if err != nil {
		return exceptions.NewLicenseError("Failed to call monalisa_set_license: %v", err)
	}

	if ret != 0 {
		return exceptions.NewLicenseError("License validation failed with code: %d", ret)
	}

	keyHex, err := s.extractLicenseKey()
	if err != nil {
		return err
	}

	keyBytes, _ := hex.DecodeString(keyHex)

	decoded, _ := base64.StdEncoding.DecodeString(licStr)
	re := regexp.MustCompile(`DCID-[A-Z0-9]+-[A-Z0-9]+-\d{8}-\d{6}-[A-Z0-9]+-\d{10}-[A-Z0-9]+`)
	match := re.Find(decoded)

	var kid uuid.UUID
	if match != nil {
		kid = uuid.NewSHA1(uuid.NameSpaceDNS, match)
	} else {
		kid = uuid.Nil
	}

	key := &types.Key{
		KID:  kid[:],
		Key:  keyBytes,
		Type: types.KeyTypeContent,
	}

	s.keys = append(s.keys, key)
	return nil
}

func (s *Session) GetKeys(keyType types.KeyType) []*types.Key {
	result := make([]*types.Key, 0)
	for _, key := range s.keys {
		if key.Type == keyType {
			result = append(result, key)
		}
	}
	return result
}

func (s *Session) Cleanup() {
	s.keys = nil
	s.instance = nil
	s.memory = nil
}

func (s *Session) extractLicenseKey() (string, error) {
	data := s.memory.UnsafeData(s.store)
	size := s.memory.DataSize(s.store)

	if LicenseKeyOffset+LicenseKeyLength > size {
		return "", exceptions.NewLicenseError("License key offset beyond memory bounds")
	}

	keyBytes := make([]byte, LicenseKeyLength)
	copy(keyBytes, data[LicenseKeyOffset:LicenseKeyOffset+LicenseKeyLength])
	return hex.EncodeToString(keyBytes), nil
}

func (s *Session) ccallInt(funcName string, args ...interface{}) (int, error) {
	stack := int32(0)
	convertedArgs := make([]interface{}, 0)

	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			if stack == 0 {
				result, _ := s.exports["stackSave"].Call(s.store)
				stack = result.(int32)
			}
			maxLen := (len(v) << 2) + 1
			result, _ := s.exports["stackAlloc"].Call(s.store, maxLen)
			ptr := result.(int32)
			s.stringToUTF8(v, int(ptr), maxLen)
			convertedArgs = append(convertedArgs, ptr)
		case int:
			convertedArgs = append(convertedArgs, int32(v))
		case int32:
			convertedArgs = append(convertedArgs, v)
		default:
			convertedArgs = append(convertedArgs, v)
		}
	}

	fn := s.exports[funcName]
	result, err := fn.Call(s.store, convertedArgs...)
	if err != nil {
		return 0, err
	}

	if stack != 0 {
		s.exports["stackRestore"].Call(s.store, stack)
	}

	if result == nil {
		return 0, nil
	}
	return int(result.(int32)), nil
}

func (s *Session) writeI32(addr, value int) {
	data := s.memory.UnsafeData(s.store)
	offset := addr
	data[offset] = byte(value)
	data[offset+1] = byte(value >> 8)
	data[offset+2] = byte(value >> 16)
	data[offset+3] = byte(value >> 24)
}

func (s *Session) stringToUTF8(str string, ptr, maxLen int) {
	encoded := []byte(str)
	writeLen := len(encoded)
	if writeLen > maxLen-1 {
		writeLen = maxLen - 1
	}

	data := s.memory.UnsafeData(s.store)
	copy(data[ptr:], encoded[:writeLen])
	data[ptr+writeLen] = 0
}

func (s *Session) getExport(name string) *wasmtime.Extern {
	return s.instance.GetExport(s.store, name)
}

func (s *Session) buildImports() ([]wasmtime.AsExtern, error) {
	envStrings := []string{
		"USER=web_user",
		"LOGNAME=web_user",
		"PATH=/",
		"PWD=/",
		"HOME=/home/web_user",
		"LANG=zh_CN.UTF-8",
		"_=./this.program",
	}

	noOp := func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		return []wasmtime.Val{wasmtime.ValI32(0)}, nil
	}

	noOpVoid := func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		return []wasmtime.Val{}, nil
	}

	emscriptenMemcpyBig := func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		dest := args[0].I32()
		src := args[1].I32()
		num := args[2].I32()

		mem := s.memory.UnsafeData(s.store)
		copy(mem[dest:dest+num], mem[src:src+num])

		return []wasmtime.Val{wasmtime.ValI32(dest)}, nil
	}

	environGet := func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		environPtr := args[0].I32()
		environBuf := args[1].I32()

		bufSize := int32(0)
		for i, str := range envStrings {
			ptr := environBuf + bufSize
			s.writeI32(int(environPtr+int32(i*4)), int(ptr))
			s.writeASCII(str, int(ptr))
			bufSize += int32(len(str) + 1)
		}
		return []wasmtime.Val{wasmtime.ValI32(0)}, nil
	}

	environSizesGet := func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		pEnvironCount := args[0].I32()
		pEnvironBufSize := args[1].I32()

		s.writeI32(int(pEnvironCount), len(envStrings))
		bufSize := 0
		for _, str := range envStrings {
			bufSize += len(str) + 1
		}
		s.writeI32(int(pEnvironBufSize), bufSize)
		return []wasmtime.Val{wasmtime.ValI32(0)}, nil
	}

	imports := []wasmtime.AsExtern{
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), noOp),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), noOp),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), noOp),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), noOp),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), noOp),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), noOp),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), noOp),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), noOp),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), noOp),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{},
		), noOpVoid),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), noOp),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), emscriptenMemcpyBig),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), noOp),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), environGet),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), environSizesGet),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), noOp),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), noOp),
		wasmtime.NewFunc(s.store, wasmtime.NewFuncType(
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
		), func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
			return []wasmtime.Val{wasmtime.ValI32(1)}, nil
		}),
		s.memory,
	}

	return imports, nil
}

func (s *Session) writeASCII(str string, ptr int) {
	data := s.memory.UnsafeData(s.store)
	for i, ch := range []byte(str) {
		data[ptr+i] = ch
	}
	data[ptr+len(str)] = 0
}
