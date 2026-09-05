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
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
	"github.com/odvcencio/gotreesitter"
)

// MaxASTFileSize caps the file size for AST parsing before falling back to line diffing.
const MaxASTFileSize = 400 * 1024

// DiffOptions configures parsing limits, comment handling, and output options.
type DiffOptions struct {
	ParseErrorLimit      int
	DisableErrorFallback bool
	DisableSizeLimit     bool
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
	SrcAST      *treesitter.ASTNode
	DstAST      *treesitter.ASTNode
	MatchResult *engine.MatchResult
	EditScript  *actions.EditScript
	Envelope    *serialize.Envelope
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

	rulesA := rules.Get(langA.Name)
	rulesB := rules.Get(langB.Name)

	var (
		srcAST      *treesitter.ASTNode
		dstAST      *treesitter.ASTNode
		srcTree     *gotreesitter.Tree
		dstTree     *gotreesitter.Tree
		srcComments []comments.CommentBlock
		dstComments []comments.CommentBlock
		wg          sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		srcAST, srcTree, _ = treesitter.ParseWithLanguageAndTree(srcBytes, langA)
		if srcTree != nil && !opts.IgnoreComments {
			srcComments = comments.ExtractComments(srcTree.RootNode(), srcBytes, langA, rulesA)
		}
	}()
	go func() {
		defer wg.Done()
		dstAST, dstTree, _ = treesitter.ParseWithLanguageAndTree(dstBytes, langB)
		if dstTree != nil && !opts.IgnoreComments {
			dstComments = comments.ExtractComments(dstTree.RootNode(), dstBytes, langB, rulesB)
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

	var (
		matchResult *engine.MatchResult
		commentRes  *comments.DiffResult
		matchWg     sync.WaitGroup
	)

	matchWg.Add(1)
	go func() {
		defer matchWg.Done()
		matchResult = engine.Match(srcAST, dstAST, srcBytes, dstBytes, part)
	}()

	if !opts.IgnoreComments && (len(srcComments) > 0 || len(dstComments) > 0) {
		matchWg.Add(1)
		go func() {
			defer matchWg.Done()
			commentRes = comments.DiffComments(srcComments, dstComments)
		}()
	}

	matchWg.Wait()

	es := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)

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
