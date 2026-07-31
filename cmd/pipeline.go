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

func computeDiff(fileA, fileB string) (*diffResult, error) {
	srcBytes, err := os.ReadFile(fileA)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", fileA, err)
	}
	dstBytes, err := os.ReadFile(fileB)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", fileB, err)
	}

	srcAST, err := treesitter.Parse(srcBytes, fileA)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", fileA, err)
	}
	dstAST, err := treesitter.Parse(dstBytes, fileB)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", fileB, err)
	}

	matchResult := engine.Match(srcAST, dstAST)
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
