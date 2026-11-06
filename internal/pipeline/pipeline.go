package pipeline

import (
	"bytes"
	"cmp"
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
	ForceBinary          bool
	ForceText            bool
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
	if !envOpts.SignificantWhitespace {
		envOpts.SignificantWhitespace = treesitter.DetectSignificantWhitespace(srcFile, dstFile)
	}

	if opts.ForceBinary || ((IsBinary(srcBytes) || IsBinary(dstBytes)) && !opts.ForceText) {
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

	fallbackResult := func(reason serialize.FallbackReason, annotation string) (*DiffResult, error) {
		env := serialize.BuildLineDiffEnvelopeWithOptions(srcBytes, dstBytes, envOpts)
		env.FallbackReason = reason
		env.FallbackAnnotation = annotation
		return &DiffResult{
			SrcBytes: srcBytes,
			DstBytes: dstBytes,
			SrcFile:  srcFile,
			DstFile:  dstFile,
			Envelope: env,
		}, nil
	}

	langA, _ := treesitter.DetectLanguage(srcFile)
	langB, _ := treesitter.DetectLanguage(dstFile)

	if langA == nil && len(srcBytes) == 0 {
		langA = langB
	}
	if langB == nil && len(dstBytes) == 0 {
		langB = langA
	}

	// If either language is unknown or languages differ (cross-language diff),
	// fall back to clean line diffing without a terminal parse error annotation.
	if langA == nil || langB == nil || langA.Name != langB.Name {
		if langA == nil || langB == nil {
			return fallbackResult(serialize.FallbackReasonUnsupportedLanguage, "")
		}
		return fallbackResult(serialize.FallbackReasonMismatchedLanguages, "")
	}

	if opts.IsConflict || (HasConflictMarkers(srcBytes) || HasConflictMarkers(dstBytes)) {
		return fallbackResult(serialize.FallbackReasonConflict, serialize.FallbackAnnotationConflict)
	}

	maxSize := cmp.Or(opts.MaxASTFileSize, MaxASTFileSize)
	maxLines := cmp.Or(opts.MaxASTFileLines, MaxASTFileLines)

	exceedsSize := !opts.DisableSizeLimit && (len(srcBytes) > maxSize || len(dstBytes) > maxSize)
	if exceedsSize {
		return fallbackResult(
			serialize.FallbackReasonFileSizeLimit,
			serialize.FormatFallbackAnnotationSizeLimit(max(len(srcBytes), len(dstBytes))),
		)
	}

	if !opts.DisableLineLimit && maxLines > 0 {
		srcLines := countFileLines(srcBytes)
		dstLines := countFileLines(dstBytes)
		if srcLines > maxLines || dstLines > maxLines {
			return fallbackResult(
				serialize.FallbackReasonLineLimit,
				serialize.FormatFallbackAnnotationLineLimit(max(srcLines, dstLines)),
			)
		}
	}

	var (
		srcAST      *treesitter.ASTNode
		dstAST      *treesitter.ASTNode
		srcComments []comments.CommentBlock
		dstComments []comments.CommentBlock
		wg          sync.WaitGroup
	)

	wg.Go(func() {
		srcAST, _ = treesitter.ParseWithLanguage(srcBytes, langA.Name)
		if !opts.IgnoreComments && srcAST != nil {
			srcComments = comments.ExtractComments(srcAST)
		}
	})
	wg.Go(func() {
		dstAST, _ = treesitter.ParseWithLanguage(dstBytes, langB.Name)
		if !opts.IgnoreComments && dstAST != nil {
			dstComments = comments.ExtractComments(dstAST)
		}
	})

	part := engine.NewLinePartition(srcBytes, dstBytes)
	wg.Wait()

	hasParseError := !opts.DisableErrorFallback && ((srcAST != nil && srcAST.ParseErrorCount > opts.ParseErrorLimit) ||
		(dstAST != nil && dstAST.ParseErrorCount > opts.ParseErrorLimit))

	if srcAST == nil || dstAST == nil || hasParseError {
		return fallbackResult(serialize.FallbackReasonParseError, serialize.FallbackAnnotationParseError)
	}

	matchResult := engine.Match(srcAST, dstAST, srcBytes, dstBytes, part)

	var (
		commentRes *comments.DiffResult
		es         *actions.EditScript
		wgPost     sync.WaitGroup
	)

	wgPost.Go(func() {
		es = actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
	})

	if !opts.IgnoreComments && (len(srcComments) > 0 || len(dstComments) > 0) {
		wgPost.Go(func() {
			commentRes = comments.DiffComments(srcComments, dstComments, matchResult.Mappings)
		})
	}

	wgPost.Wait()

	if commentRes != nil {
		for _, act := range commentRes.Actions {
			es.Add(act)
		}
		if len(commentRes.LineMappings) > 0 {
			envOpts.CommentLineMappings = commentRes.LineMappings
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

// countFileLines returns the line count, including a final line without a trailing newline.
func countFileLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}
