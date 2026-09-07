package treesitter

/*
#cgo CFLAGS: -I${SRCDIR}/../../native/bridge/include -O3
#cgo LDFLAGS: -L${SRCDIR}/../../native/bridge/lib -ldiffmantic_grammars -lstdc++
#include "bridge.h"
#include "../../native/bridge/src/bridge.c"
#include <dlfcn.h>

extern const TSLanguage* diffmantic_get_native_language(const char* name);

typedef void* (*tree_sitter_lang_fn)(void);

static void* get_ts_lang(const char* lib_path, const char* symbol_name) {
    void* handle = dlopen(lib_path, RTLD_NOW);
    if (!handle) return NULL;
    tree_sitter_lang_fn fn = (tree_sitter_lang_fn)dlsym(handle, symbol_name);
    if (!fn) {
        dlclose(handle);
        return NULL;
    }
    return fn();
}

static uint32_t get_ts_lang_symbol_count(const void* ts_lang_ptr) {
    return ts_language_symbol_count((const TSLanguage*)ts_lang_ptr);
}

static const char* get_ts_lang_symbol_name(const void* ts_lang_ptr, uint16_t symbol_id) {
    return ts_language_symbol_name((const TSLanguage*)ts_lang_ptr, symbol_id);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"unsafe"
)

type cachedLang struct {
	ptr     unsafe.Pointer
	symbols []string
}

var (
	nativeLangCache = make(map[string]cachedLang)
	nativeLangMu    sync.RWMutex
)

func getLanguageSymbols(langName string) (unsafe.Pointer, []string, error) {
	if langName == "" {
		return nil, nil, errors.New("empty language name")
	}

	nativeLangMu.RLock()
	if entry, ok := nativeLangCache[langName]; ok {
		nativeLangMu.RUnlock()
		return entry.ptr, entry.symbols, nil
	}
	nativeLangMu.RUnlock()

	nativeLangMu.Lock()
	defer nativeLangMu.Unlock()

	if entry, ok := nativeLangCache[langName]; ok {
		return entry.ptr, entry.symbols, nil
	}

	ptr, err := GetNativeLanguage(langName)
	if err != nil || ptr == nil {
		return nil, nil, fmt.Errorf("native parser not available for %s: %w", langName, err)
	}

	symbols := NativeLanguageSymbols(ptr)
	nativeLangCache[langName] = cachedLang{ptr: ptr, symbols: symbols}
	return ptr, symbols, nil
}

// ParseWithLanguage parses source bytes for a given language name.
func ParseWithLanguage(src []byte, langName string) (*ASTNode, error) {
	ptr, symbols, err := getLanguageSymbols(langName)
	if err != nil {
		return nil, err
	}
	return ParseWithNativeFlatBuffer(src, ptr, langName, symbols)
}

// ParseForPipeline parses via the native flat-buffer bridge, returning the root AST,
// raw flat nodes, and symbol table for comment extraction in the diff pipeline.
func ParseForPipeline(src []byte, langName string) (*ASTNode, []FlatNode, []string, error) {
	ptr, symbols, err := getLanguageSymbols(langName)
	if err != nil {
		return nil, nil, nil, err
	}
	ast, flatNodes, err := parseWithNativeFlatBufferKeepNodes(src, ptr, langName, symbols)
	return ast, flatNodes, symbols, err
}

// parseWithNativeFlatBufferKeepNodes is like ParseWithNativeFlatBuffer but also returns
// a Go-owned copy of the flat node array for comment extraction.
func parseWithNativeFlatBufferKeepNodes(src []byte, tsLangPtr unsafe.Pointer, langName string, symbols []string) (*ASTNode, []FlatNode, error) {
	if tsLangPtr == nil {
		return nil, nil, errors.New("nil native language pointer")
	}

	if len(symbols) == 0 {
		symbols = NativeLanguageSymbols(tsLangPtr)
	}

	var srcPtr *C.uint8_t
	if len(src) > 0 {
		srcPtr = (*C.uint8_t)(unsafe.Pointer(&src[0]))
	}

	res := C.parse_to_flat_ast(
		srcPtr,
		C.size_t(len(src)),
		tsLangPtr,
	)

	if res.error_code != 0 {
		return nil, nil, fmt.Errorf("native flat tree-sitter parse failed (code %d)", int(res.error_code))
	}

	defer C.free_flat_ast(res)

	var nodes []FlatNode
	if res.node_count > 0 && res.nodes_ptr != nil {
		nodes = unsafe.Slice((*FlatNode)(unsafe.Pointer(res.nodes_ptr)), int(res.node_count))
	}

	// Copy flat nodes into Go-owned memory for comment extraction (C memory freed on return).
	goNodes := slices.Clone(nodes)

	root := IngestFlatAST(nodes, symbols, src, langName)

	return root, goNodes, nil
}

type FlatASTResult = C.FlatASTResult

// GetNativeLanguage retrieves the statically linked native Tree-sitter language for any of the 16 core languages (18 grammars).
func GetNativeLanguage(langName string) (unsafe.Pointer, error) {
	cName := C.CString(langName)
	defer C.free(unsafe.Pointer(cName))

	ptr := unsafe.Pointer(C.diffmantic_get_native_language(cName))
	if ptr == nil {
		return nil, fmt.Errorf("unsupported or unregistered native language: %s", langName)
	}
	return ptr, nil
}

// LoadNativeLanguage dynamically loads a Tree-sitter grammar function pointer from a shared library.
func LoadNativeLanguage(libPath, symbolName string) (unsafe.Pointer, error) {
	cPath := C.CString(libPath)
	defer C.free(unsafe.Pointer(cPath))
	cSym := C.CString(symbolName)
	defer C.free(unsafe.Pointer(cSym))

	ptr := C.get_ts_lang(cPath, cSym)
	if ptr == nil {
		return nil, fmt.Errorf("failed to load symbol %s from %s", symbolName, libPath)
	}
	return ptr, nil
}

// NativeLanguageSymbols returns the slice of symbol names defined by the native Tree-sitter language.
func NativeLanguageSymbols(tsLangPtr unsafe.Pointer) []string {
	if tsLangPtr == nil {
		return nil
	}
	count := int(C.get_ts_lang_symbol_count(tsLangPtr))
	symbols := make([]string, count)
	for i := range count {
		cName := C.get_ts_lang_symbol_name(tsLangPtr, C.uint16_t(i))
		if cName != nil {
			symbols[i] = C.GoString(cName)
		}
	}
	return symbols
}

// ParseWithNativeFlatBuffer parses source code using the native Tree-sitter engine
// and converts the resulting flat buffer into an ASTNode tree.
func ParseWithNativeFlatBuffer(src []byte, tsLangPtr unsafe.Pointer, langName string, symbols []string) (*ASTNode, error) {
	if tsLangPtr == nil {
		return nil, errors.New("nil native language pointer")
	}

	if len(symbols) == 0 {
		symbols = NativeLanguageSymbols(tsLangPtr)
	}

	var srcPtr *C.uint8_t
	if len(src) > 0 {
		srcPtr = (*C.uint8_t)(unsafe.Pointer(&src[0]))
	}

	res := C.parse_to_flat_ast(
		srcPtr,
		C.size_t(len(src)),
		tsLangPtr,
	)

	if res.error_code != 0 {
		return nil, fmt.Errorf("native flat tree-sitter parse failed (code %d)", int(res.error_code))
	}

	// Release the C flat buffer on return, even if ingestion panics.
	defer C.free_flat_ast(res)

	var nodes []FlatNode
	if res.node_count > 0 && res.nodes_ptr != nil {
		nodes = unsafe.Slice((*FlatNode)(unsafe.Pointer(res.nodes_ptr)), int(res.node_count))
	}

	root := IngestFlatAST(nodes, symbols, src, langName)

	return root, nil
}
