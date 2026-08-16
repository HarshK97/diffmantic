package pipeline

import (
	"bytes"
	"fmt"
	"os"
	"sync"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/postprocess"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

const MaxASTFileSize = 400 * 1024

type DiffOptions struct {
	ParseErrorLimit      int
	DisableErrorFallback bool
	DisableSizeLimit     bool
	IsConflict           bool
	EnvelopeOpts         serialize.EnvelopeOptions
}

type DiffResult struct {
	SrcBytes    []byte
	DstBytes    []byte
	SrcFile     string
	DstFile     string
	SrcAST      *treesitter.ASTNode
	DstAST      *treesitter.ASTNode
	MatchResult *engine.MatchResult
	EditScript  *actions.EditScript
	Envelope    *serialize.Envelope
}

func HasConflictMarkers(data []byte) bool {
	hasStart := bytes.HasPrefix(data, []byte("<<<<<<<")) || bytes.Contains(data, []byte("\n<<<<<<<"))
	hasEnd := bytes.Contains(data, []byte("\n>>>>>>>")) || bytes.Contains(data, []byte(">>>>>>>\n"))
	return hasStart && hasEnd
}

// Run executes the diffmantic semantic diff pipeline on in-memory buffers.
// It parses ASTs and computes line partitions concurrently.
func Run(srcBytes, dstBytes []byte, srcFile, dstFile string, opts DiffOptions) (*DiffResult, error) {
	envOpts := opts.EnvelopeOpts
	if !envOpts.IncludeActions && !envOpts.IncludeAlignment && !envOpts.IncludeHighlights {
		envOpts = serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		}
	}

	if opts.IsConflict || (HasConflictMarkers(srcBytes) || HasConflictMarkers(dstBytes)) ||
		(!opts.DisableSizeLimit && (len(srcBytes) > MaxASTFileSize || len(dstBytes) > MaxASTFileSize)) {
		return &DiffResult{
			SrcBytes: srcBytes,
			DstBytes: dstBytes,
			SrcFile:  srcFile,
			DstFile:  dstFile,
			Envelope: serialize.BuildLineDiffEnvelopeWithOptions(srcBytes, dstBytes, envOpts),
		}, nil
	}

	langA, _ := treesitter.DetectLanguage(srcFile)
	langB, _ := treesitter.DetectLanguage(dstFile)

	// Fall back to line diff if tree-sitter cannot detect the language.
	if langA == nil && langB == nil {
		return &DiffResult{
			SrcBytes: srcBytes,
			DstBytes: dstBytes,
			SrcFile:  srcFile,
			DstFile:  dstFile,
			Envelope: serialize.BuildLineDiffEnvelopeWithOptions(srcBytes, dstBytes, envOpts),
		}, nil
	}

	if langA == nil {
		langA = langB
	}
	if langB == nil {
		langB = langA
	}

	var (
		srcAST *treesitter.ASTNode
		dstAST *treesitter.ASTNode
		part   *engine.LinePartition
		wg     sync.WaitGroup
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		srcAST, _ = treesitter.ParseWithLanguage(srcBytes, langA)
	}()
	go func() {
		defer wg.Done()
		dstAST, _ = treesitter.ParseWithLanguage(dstBytes, langB)
	}()
	go func() {
		defer wg.Done()
		part = engine.NewLinePartition(srcBytes, dstBytes)
	}()
	wg.Wait()

	if srcAST == nil || dstAST == nil || (!opts.DisableErrorFallback && (srcAST.ParseErrorCount > opts.ParseErrorLimit || dstAST.ParseErrorCount > opts.ParseErrorLimit)) {
		return &DiffResult{
			SrcBytes: srcBytes,
			DstBytes: dstBytes,
			SrcFile:  srcFile,
			DstFile:  dstFile,
			Envelope: serialize.BuildLineDiffEnvelopeWithOptions(srcBytes, dstBytes, envOpts),
		}, nil
	}

	matchResult := engine.Match(srcAST, dstAST, srcBytes, dstBytes, part)
	es := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
	es = postprocess.Run(es, matchResult.Mappings, srcAST, dstAST)

	env, err := serialize.BuildEnvelopeWithOptions(es, matchResult.Mappings, srcAST, dstAST, srcBytes, dstBytes, envOpts)
	if err != nil {
		return nil, fmt.Errorf("building envelope: %w", err)
	}

	return &DiffResult{
		SrcBytes:    srcBytes,
		DstBytes:    dstBytes,
		SrcFile:     srcFile,
		DstFile:     dstFile,
		SrcAST:      srcAST,
		DstAST:      dstAST,
		MatchResult: matchResult,
		EditScript:  es,
		Envelope:    env,
	}, nil
}

// RunFiles reads two files from disk and executes the diffmantic pipeline.
func RunFiles(fileA, fileB string, opts DiffOptions) (*DiffResult, error) {
	srcBytes, err := os.ReadFile(fileA)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", fileA, err)
	}
	dstBytes, err := os.ReadFile(fileB)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", fileB, err)
	}
	return Run(srcBytes, dstBytes, fileA, fileB, opts)
}
