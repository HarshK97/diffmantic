package cmd

import (
	"fmt"
	"os"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/postprocess"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

type diffResult struct {
	SrcBytes    []byte
	DstBytes    []byte
	SrcFile     string
	DstFile     string
	MatchResult *engine.MatchResult
	EditScript  *actions.EditScript
	Envelope    *serialize.Envelope
}

const MaxASTFileSize = 400 * 1024

func computeDiff(fileA, fileB string) (*diffResult, error) {
	srcBytes, err := os.ReadFile(fileA)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", fileA, err)
	}
	dstBytes, err := os.ReadFile(fileB)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", fileB, err)
	}

	if len(srcBytes) > MaxASTFileSize || len(dstBytes) > MaxASTFileSize {
		return &diffResult{
			SrcBytes: srcBytes,
			DstBytes: dstBytes,
			SrcFile:  fileA,
			DstFile:  fileB,
			Envelope: serialize.BuildLineDiffEnvelope(srcBytes, dstBytes),
		}, nil
	}

	langA, _ := treesitter.DetectLanguage(fileA)
	langB, _ := treesitter.DetectLanguage(fileB)

	// Fall back to line diff if tree-sitter can't parse the file.
	if langA == nil && langB == nil {
		return &diffResult{
			SrcBytes: srcBytes,
			DstBytes: dstBytes,
			SrcFile:  fileA,
			DstFile:  fileB,
			Envelope: serialize.BuildLineDiffEnvelope(srcBytes, dstBytes),
		}, nil
	}

	if langA == nil {
		langA = langB
	}
	if langB == nil {
		langB = langA
	}

	srcAST, _ := treesitter.ParseWithLanguage(srcBytes, langA)
	dstAST, _ := treesitter.ParseWithLanguage(dstBytes, langB)
	if srcAST == nil || dstAST == nil {
		return &diffResult{
			SrcBytes: srcBytes,
			DstBytes: dstBytes,
			SrcFile:  fileA,
			DstFile:  fileB,
			Envelope: serialize.BuildLineDiffEnvelope(srcBytes, dstBytes),
		}, nil
	}

	matchResult := engine.Match(srcAST, dstAST, srcBytes, dstBytes)
	es := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
	es = postprocess.Run(es, matchResult.Mappings, srcAST, dstAST)

	env, err := serialize.BuildEnvelope(es, matchResult.Mappings, srcAST, dstAST, srcBytes, dstBytes)
	if err != nil {
		return nil, fmt.Errorf("building envelope: %w", err)
	}

	return &diffResult{
		SrcBytes:    srcBytes,
		DstBytes:    dstBytes,
		SrcFile:     fileA,
		DstFile:     fileB,
		MatchResult: matchResult,
		EditScript:  es,
		Envelope:    env,
	}, nil
}
