package pipeline

import (
	"bytes"
	"fmt"
	"os"
	"sync"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/comments"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/postprocess"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// MaxASTFileSize caps the file size for AST parsing before falling back to line diffing.
const MaxASTFileSize = 1024 * 1024

// MaxASTFileLines caps the line count for AST parsing before falling back to line diffing.
const MaxASTFileLines = 10000

// DiffOptions configures parsing limits, comment handling, and output options.
type DiffOptions struct {
	ParseErrorLimit      int
	DisableErrorFallback bool
	DisableSizeLimit     bool
	MaxASTFileSize       int
	DisableLineLimit     bool
	MaxASTFileLines      int
	IsConflict           bool
	IgnoreComments       bool
	EnvelopeOpts         serialize.EnvelopeOptions
}

// DiffResult holds the computed ASTs, mappings, edit script, and serialized envelope.
type DiffResult struct {
	SrcBytes    []byte
	DstBytes    []byte
	SrcFile     string
	DstFile     string
	IsBinary    bool
	SrcAST      *treesitter.ASTNode
	DstAST      *treesitter.ASTNode
	MatchResult *engine.MatchResult
	EditScript  *actions.EditScript
	Envelope    *serialize.Envelope
}

// IsBinary detects whether a byte buffer contains binary data (null bytes in the first 8000 bytes).
func IsBinary(data []byte) bool {
	sample := data
	if len(sample) > 8000 {
		sample = sample[:8000]
	}
	return bytes.IndexByte(sample, 0) != -1
}

// HasConflictMarkers checks if the buffer contains Git merge conflict markers.
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

	if IsBinary(srcBytes) || IsBinary(dstBytes) {
		return &DiffResult{
			SrcBytes: srcBytes,
			DstBytes: dstBytes,
			SrcFile:  srcFile,
			DstFile:  dstFile,
			IsBinary: true,
			Envelope: &serialize.Envelope{
				Version:  serialize.SchemaVersion,
				IsBinary: true,
			},
		}, nil
	}

	maxSize := MaxASTFileSize
	if opts.MaxASTFileSize > 0 {
		maxSize = opts.MaxASTFileSize
	}

	maxLines := MaxASTFileLines
	if opts.MaxASTFileLines > 0 {
		maxLines = opts.MaxASTFileLines
	}

	exceedsLines := !opts.DisableLineLimit && maxLines > 0 &&
		(bytes.Count(srcBytes, []byte{'\n'}) > maxLines || bytes.Count(dstBytes, []byte{'\n'}) > maxLines)

	exceedsSize := !opts.DisableSizeLimit && (len(srcBytes) > maxSize || len(dstBytes) > maxSize)

	if opts.IsConflict || (HasConflictMarkers(srcBytes) || HasConflictMarkers(dstBytes)) ||
		exceedsSize || exceedsLines {
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

	// If tree-sitter doesn't support the language, fall back to line diffing.
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
		srcAST      *treesitter.ASTNode
		dstAST      *treesitter.ASTNode
		srcComments []comments.CommentBlock
		dstComments []comments.CommentBlock
		wg          sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		var (
			srcFlatNodes []treesitter.FlatNode
			srcSymbols   []string
		)
		srcAST, srcFlatNodes, srcSymbols, _ = treesitter.ParseForPipeline(srcBytes, langA.Name)
		if !opts.IgnoreComments && len(srcFlatNodes) > 0 {
			srcComments = comments.ExtractComments(srcFlatNodes, srcSymbols, srcBytes, langA.Name)
			if srcAST != nil {
				comments.BindASTNodes(srcComments, srcAST)
			}
		}
	}()
	go func() {
		defer wg.Done()
		var (
			dstFlatNodes []treesitter.FlatNode
			dstSymbols   []string
		)
		dstAST, dstFlatNodes, dstSymbols, _ = treesitter.ParseForPipeline(dstBytes, langB.Name)
		if !opts.IgnoreComments && len(dstFlatNodes) > 0 {
			dstComments = comments.ExtractComments(dstFlatNodes, dstSymbols, dstBytes, langB.Name)
			if dstAST != nil {
				comments.BindASTNodes(dstComments, dstAST)
			}
		}
	}()

	part := engine.NewLinePartition(srcBytes, dstBytes)
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

	var (
		commentRes *comments.DiffResult
		es         *actions.EditScript
		wgPost     sync.WaitGroup
	)

	wgPost.Add(1)
	go func() {
		defer wgPost.Done()
		es = actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
	}()

	if !opts.IgnoreComments && (len(srcComments) > 0 || len(dstComments) > 0) {
		wgPost.Add(1)
		go func() {
			defer wgPost.Done()
			commentRes = comments.DiffComments(srcComments, dstComments, matchResult.Mappings)
		}()
	}

	wgPost.Wait()

	if commentRes != nil && len(commentRes.Actions) > 0 {
		for _, act := range commentRes.Actions {
			es.Add(act)
		}
	}

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
