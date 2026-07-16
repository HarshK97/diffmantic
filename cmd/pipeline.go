package cmd

import (
	"fmt"
	"os"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/postprocess"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"golang.org/x/sync/errgroup"
)

type diffResult struct {
	SrcBytes    []byte
	DstBytes    []byte
	SrcFile     string
	DstFile     string
	SrcAST      *treesitter.ASTNode
	DstAST      *treesitter.ASTNode
	MatchResult *engine.MatchResult
	EditScript  *actions.EditScript
}

func computeDiff(fileA, fileB string) (*diffResult, error) {
	var (
		srcBytes []byte
		dstBytes []byte
		srcAST   *treesitter.ASTNode
		dstAST   *treesitter.ASTNode
	)

	g := new(errgroup.Group)
	g.Go(func() error {
		data, err := os.ReadFile(fileA)
		if err != nil {
			return fmt.Errorf("reading %s: %w", fileA, err)
		}
		srcBytes = data
		return nil
	})
	g.Go(func() error {
		data, err := os.ReadFile(fileB)
		if err != nil {
			return fmt.Errorf("reading %s: %w", fileB, err)
		}
		dstBytes = data
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	g = new(errgroup.Group)
	g.Go(func() error {
		parsed, err := treesitter.Parse(srcBytes, fileA)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", fileA, err)
		}
		srcAST = parsed
		return nil
	})
	g.Go(func() error {
		parsed, err := treesitter.Parse(dstBytes, fileB)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", fileB, err)
		}
		dstAST = parsed
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	matchResult := engine.Match(srcAST, dstAST)
	es := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
	es = postprocess.Run(es, matchResult.Mappings, srcAST, dstAST)

	return &diffResult{
		SrcBytes:    srcBytes,
		DstBytes:    dstBytes,
		SrcFile:     fileA,
		DstFile:     fileB,
		SrcAST:      srcAST,
		DstAST:      dstAST,
		MatchResult: matchResult,
		EditScript:  es,
	}, nil
}
