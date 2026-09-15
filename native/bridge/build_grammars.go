package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
)

type RepoInfo struct {
	Repo    string `json:"repo"`
	Version string `json:"version"`
}

type GrammarSpec struct {
	Name     string
	SubDir   string
	ExtraInc string
}

type TargetSpec struct {
	Name      string // "linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64", "windows_amd64"
	ZigTarget string // "x86_64-linux-musl", "aarch64-linux-musl", "x86_64-macos", "aarch64-macos", "x86_64-windows-gnu"
	OS        string // "linux", "darwin", "windows"
}

var releaseTargets = []TargetSpec{
	{Name: "linux_amd64", ZigTarget: "x86_64-linux-musl", OS: "linux"},
	{Name: "linux_arm64", ZigTarget: "aarch64-linux-musl", OS: "linux"},
	{Name: "darwin_amd64", ZigTarget: "x86_64-macos", OS: "darwin"},
	{Name: "darwin_arm64", ZigTarget: "aarch64-macos", OS: "darwin"},
	{Name: "windows_amd64", ZigTarget: "x86_64-windows-gnu", OS: "windows"},
}

var grammars = []GrammarSpec{
	{Name: "c", SubDir: "src"},
	{Name: "cpp", SubDir: "src"},
	{Name: "go", SubDir: "src"},
	{Name: "rust", SubDir: "src"},
	{Name: "python", SubDir: "src"},
	{Name: "javascript", SubDir: "src"},
	{Name: "typescript", SubDir: "typescript/src", ExtraInc: "common"},
	{Name: "tsx", SubDir: "tsx/src", ExtraInc: "common"},
	{Name: "java", SubDir: "src"},
	{Name: "php", SubDir: "php/src", ExtraInc: "common"},
	{Name: "ruby", SubDir: "src"},
	{Name: "lua", SubDir: "src"},
	{Name: "zig", SubDir: "src"},
	{Name: "css", SubDir: "src"},
	{Name: "html", SubDir: "src"},
	{Name: "json", SubDir: "src"},
	{Name: "toml", SubDir: "src"},
	{Name: "yaml", SubDir: "src"},
}

type compileJob struct {
	desc     string
	compiler string
	args     []string
	output   string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		crossAllFlag bool
		targetFlag   string
	)
	flag.BoolVar(&crossAllFlag, "cross-all", false, "Compile static libraries for all release targets using Zig")
	flag.StringVar(&targetFlag, "target", "", "Compile static library for a specific target (e.g. linux_amd64)")
	flag.Parse()

	_, currentFile, _, _ := runtime.Caller(0)
	bridgeDir := filepath.Dir(currentFile)
	grammarsDir := filepath.Join(bridgeDir, "grammars")
	libDir := filepath.Join(bridgeDir, "lib")

	if err := os.MkdirAll(grammarsDir, 0o755); err != nil {
		return fmt.Errorf("creating grammars dir: %w", err)
	}
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		return fmt.Errorf("creating lib dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(bridgeDir, "include", "tree_sitter"), 0o755); err != nil {
		return fmt.Errorf("creating include dir: %w", err)
	}

	manifestPath := filepath.Join(bridgeDir, "grammars.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading grammars.json: %w", err)
	}

	var manifest map[string]RepoInfo
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("parsing grammars.json: %w", err)
	}

	if err := ensureAllRepos(bridgeDir, grammarsDir, manifest); err != nil {
		return err
	}

	if crossAllFlag {
		fmt.Println("==> Compiling native grammars for all release targets via Zig...")
		for _, target := range releaseTargets {
			fmt.Printf("\n===> Building target %s (%s)...\n", target.Name, target.ZigTarget)
			if err := compileTarget(&target, bridgeDir, grammarsDir, libDir); err != nil {
				return fmt.Errorf("compiling target %s: %w", target.Name, err)
			}
		}
		// Also ensure host library exists
		fmt.Println("\n===> Building host library for local execution...")
		if err := compileTarget(nil, bridgeDir, grammarsDir, libDir); err != nil {
			return fmt.Errorf("compiling host target: %w", err)
		}
		return nil
	}

	if targetFlag != "" {
		for _, target := range releaseTargets {
			if target.Name == targetFlag {
				fmt.Printf("==> Compiling native grammars for target %s (%s) via Zig...\n", target.Name, target.ZigTarget)
				return compileTarget(&target, bridgeDir, grammarsDir, libDir)
			}
		}
		return fmt.Errorf("unknown target %q (available: linux_amd64, linux_arm64, darwin_amd64, darwin_arm64, windows_amd64)", targetFlag)
	}

	// Default: Host build
	return compileTarget(nil, bridgeDir, grammarsDir, libDir)
}

func ensureAllRepos(bridgeDir, grammarsDir string, manifest map[string]RepoInfo) error {
	fmt.Println("==> Fetching and syncing Tree-sitter core + 18 native grammars...")
	tsInfo, ok := manifest["tree-sitter"]
	if !ok {
		return fmt.Errorf("missing 'tree-sitter' in grammars.json")
	}

	tsCoreDir := filepath.Join(grammarsDir, "tree_sitter_repo")
	if err := ensureRepo(tsInfo.Repo, tsInfo.Version, tsCoreDir); err != nil {
		return fmt.Errorf("fetching tree-sitter core: %w", err)
	}

	srcAPI := filepath.Join(tsCoreDir, "lib", "include", "tree_sitter", "api.h")
	dstAPI := filepath.Join(bridgeDir, "include", "tree_sitter", "api.h")
	if err := copyFile(srcAPI, dstAPI); err != nil {
		return fmt.Errorf("syncing api.h: %w", err)
	}

	repoCache := make(map[string]string)
	for _, g := range grammars {
		repoDirName := g.Name + "_repo"
		if g.Name == "tsx" {
			repoDirName = "typescript_repo"
		}
		repoDir := filepath.Join(grammarsDir, repoDirName)

		if _, alreadyEnsured := repoCache[repoDir]; !alreadyEnsured {
			info, ok := manifest[g.Name]
			if !ok {
				return fmt.Errorf("missing '%s' in grammars.json", g.Name)
			}
			if err := ensureRepo(info.Repo, info.Version, repoDir); err != nil {
				return fmt.Errorf("fetching grammar %s: %w", g.Name, err)
			}
			repoCache[repoDir] = info.Version
		}
	}
	return nil
}

func compileTarget(target *TargetSpec, bridgeDir, grammarsDir, libDir string) error {
	buildTmp, err := os.MkdirTemp("", "diffmantic_grammars_build_*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(buildTmp) }()

	var (
		ccCompiler  string
		ccPrefix    []string
		cxxCompiler string
		cxxPrefix   []string
		arTool      string
		arPrefix    []string
		archFlags   []string
		posixFlags  []string
		targetLib   string
	)

	if target == nil {
		// Host build
		cc, err := findTool("CC", "clang", "gcc", "cc")
		if err != nil {
			return fmt.Errorf("discovering C compiler: %w", err)
		}
		cxx, err := findTool("CXX", "clang++", "g++", "c++")
		if err != nil {
			return fmt.Errorf("discovering C++ compiler: %w", err)
		}
		ar, err := findTool("AR", "ar", "llvm-ar")
		if err != nil {
			return fmt.Errorf("discovering archiver: %w", err)
		}
		ccCompiler = cc
		cxxCompiler = cxx
		arTool = ar

		if runtime.GOOS == "darwin" && isClang(cc) {
			archFlags = []string{"-arch", "arm64", "-arch", "x86_64"}
		}
		if runtime.GOOS != "windows" {
			posixFlags = []string{"-fPIC", "-fvisibility=hidden"}
		}
		targetLib = filepath.Join(libDir, "libdiffmantic_grammars.a")
	} else {
		// Cross-compilation with Zig
		zigPath, err := exec.LookPath("zig")
		if err != nil {
			return fmt.Errorf("zig compiler not found in PATH: %w", err)
		}
		ccCompiler = zigPath
		ccPrefix = []string{"cc", "-target", target.ZigTarget}
		cxxCompiler = zigPath
		cxxPrefix = []string{"c++", "-target", target.ZigTarget}
		arTool = zigPath
		arPrefix = []string{"ar"}

		if target.OS != "windows" {
			posixFlags = []string{"-fPIC", "-fvisibility=hidden"}
		}
		targetDir := filepath.Join(libDir, target.Name)
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return fmt.Errorf("creating target dir %s: %w", targetDir, err)
		}
		targetLib = filepath.Join(targetDir, "libdiffmantic_grammars.a")
	}

	tsCoreDir := filepath.Join(grammarsDir, "tree_sitter_repo")
	coreObj := filepath.Join(buildTmp, "tree_sitter.o")
	coreArgs := append([]string{}, ccPrefix...)
	coreArgs = append(coreArgs, "-O3")
	coreArgs = append(coreArgs, posixFlags...)
	coreArgs = append(coreArgs, archFlags...)
	coreArgs = append(coreArgs,
		"-I", filepath.Join(tsCoreDir, "lib", "include"),
		"-I", filepath.Join(tsCoreDir, "lib", "src"),
		"-c", filepath.Join(tsCoreDir, "lib", "src", "lib.c"),
		"-o", coreObj,
	)

	fmt.Println("  --> Compiling tree_sitter core lib.c...")
	if err := runCmd(ccCompiler, coreArgs...); err != nil {
		return fmt.Errorf("compiling tree-sitter core: %w", err)
	}

	var jobs []compileJob
	for _, g := range grammars {
		repoDirName := g.Name + "_repo"
		if g.Name == "tsx" {
			repoDirName = "typescript_repo"
		}
		repoDir := filepath.Join(grammarsDir, repoDirName)
		fullSrc := filepath.Join(repoDir, g.SubDir)

		incArgs := []string{
			"-I", fullSrc,
			"-I", filepath.Join(bridgeDir, "include"),
		}
		if g.ExtraInc != "" {
			incArgs = append(incArgs, "-I", filepath.Join(repoDir, g.ExtraInc))
		}

		baseFlags := append([]string{}, ccPrefix...)
		baseFlags = append(baseFlags, "-O3")
		baseFlags = append(baseFlags, posixFlags...)
		baseFlags = append(baseFlags, archFlags...)
		baseFlags = append(baseFlags, incArgs...)

		parserC := filepath.Join(fullSrc, "parser.c")
		if fileExists(parserC) {
			outObj := filepath.Join(buildTmp, fmt.Sprintf("%s_parser.o", g.Name))
			args := slices.Clone(baseFlags)
			args = append(args, "-c", parserC, "-o", outObj)
			jobs = append(jobs, compileJob{
				compiler: ccCompiler,
				args:     args,
				output:   outObj,
				desc:     fmt.Sprintf("%s parser.c", g.Name),
			})
		}

		scannerC := filepath.Join(fullSrc, "scanner.c")
		scannerCC := filepath.Join(fullSrc, "scanner.cc")
		if fileExists(scannerC) {
			outObj := filepath.Join(buildTmp, fmt.Sprintf("%s_scanner.o", g.Name))
			args := slices.Clone(baseFlags)
			args = append(args, "-c", scannerC, "-o", outObj)
			jobs = append(jobs, compileJob{
				compiler: ccCompiler,
				args:     args,
				output:   outObj,
				desc:     fmt.Sprintf("%s scanner.c", g.Name),
			})
		} else if fileExists(scannerCC) {
			outObj := filepath.Join(buildTmp, fmt.Sprintf("%s_scanner.o", g.Name))
			args := append([]string{}, cxxPrefix...)
			args = append(args, "-O3")
			args = append(args, posixFlags...)
			args = append(args, archFlags...)
			args = append(args, incArgs...)
			args = append(args, "-c", scannerCC, "-o", outObj)
			jobs = append(jobs, compileJob{
				compiler: cxxCompiler,
				args:     args,
				output:   outObj,
				desc:     fmt.Sprintf("%s scanner.cc", g.Name),
			})
		}
	}

	registryObj := filepath.Join(buildTmp, "registry.o")
	registryArgs := append([]string{}, ccPrefix...)
	registryArgs = append(registryArgs, "-O3")
	registryArgs = append(registryArgs, posixFlags...)
	registryArgs = append(registryArgs, archFlags...)
	registryArgs = append(registryArgs,
		"-I", filepath.Join(bridgeDir, "include"),
		"-c", filepath.Join(bridgeDir, "src", "registry.c"),
		"-o", registryObj,
	)
	jobs = append(jobs, compileJob{
		compiler: ccCompiler,
		args:     registryArgs,
		output:   registryObj,
		desc:     "grammar registry.c",
	})

	numWorkers := max(1, runtime.NumCPU())
	jobCh := make(chan compileJob, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	var (
		wg         sync.WaitGroup
		cancelOnce sync.Once
	)

	for range numWorkers {
		wg.Go(func() {
			for j := range jobCh {
				if ctx.Err() != nil {
					return
				}
				fmt.Printf("  --> Compiling %s...\n", j.desc)
				if err := runCmdContext(ctx, j.compiler, j.args...); err != nil {
					cancelOnce.Do(func() {
						cancel(fmt.Errorf("compiling %s: %w", j.desc, err))
					})
					return
				}
			}
		})
	}
	wg.Wait()

	if err := context.Cause(ctx); err != nil {
		return err
	}

	fmt.Printf("==> Creating static archive %s...\n", targetLib)
	_ = os.Remove(targetLib)

	arArgs := append([]string{}, arPrefix...)
	arArgs = append(arArgs, "rcs", targetLib, coreObj)
	for _, j := range jobs {
		arArgs = append(arArgs, j.output)
	}

	if err := runCmd(arTool, arArgs...); err != nil {
		return fmt.Errorf("creating static archive with %s: %w", arTool, err)
	}

	fmt.Printf("==> Successfully built %s!\n", targetLib)
	return nil
}

func ensureRepo(repo, version, targetDir string) error {
	if verifyRepoVersion(targetDir, version) {
		fmt.Printf("  --> Using cached %s sources (%s)\n", filepath.Base(targetDir), version)
		return nil
	}
	url := fmt.Sprintf("https://github.com/%s.git", repo)
	fmt.Printf("  --> Cloning %s (%s) from %s...\n", filepath.Base(targetDir), version, url)
	_ = os.RemoveAll(targetDir)
	if err := runCmd("git", "clone", "--depth", "1", "--branch", version, url, targetDir); err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(targetDir, ".diffmantic_version"), []byte(version), 0o644)
	return nil
}

func verifyRepoVersion(targetDir, expectedVersion string) bool {
	verFile := filepath.Join(targetDir, ".diffmantic_version")
	if data, err := os.ReadFile(verFile); err == nil {
		if strings.TrimSpace(string(data)) == expectedVersion {
			return true
		}
	}

	if dirExists(filepath.Join(targetDir, ".git")) {
		cmd := exec.Command("git", "-C", targetDir, "tag", "--points-at", "HEAD")
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := cmd.Output(); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.TrimSpace(line) == expectedVersion {
					_ = os.WriteFile(verFile, []byte(expectedVersion), 0o644)
					return true
				}
			}
		}

		cmdDesc := exec.Command("git", "-C", targetDir, "describe", "--tags")
		cmdDesc.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := cmdDesc.Output(); err == nil {
			if strings.TrimSpace(string(out)) == expectedVersion {
				_ = os.WriteFile(verFile, []byte(expectedVersion), 0o644)
				return true
			}
		}
	}
	return false
}

func runCmdContext(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = os.Environ()
	if os.Getenv("GIT_CONFIG_GLOBAL") == "" {
		cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL=/dev/null")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v failed: %w\nOutput:\n%s", name, args, err, string(out))
	}
	return nil
}

func runCmd(name string, args ...string) error {
	return runCmdContext(context.Background(), name, args...)
}

func findTool(envVar string, candidates ...string) (string, error) {
	if val := os.Getenv(envVar); val != "" {
		if path, err := exec.LookPath(val); err == nil {
			return path, nil
		}
		if fileExists(val) {
			return val, nil
		}
		return "", fmt.Errorf("tool specified in %s (%q) not found", envVar, val)
	}
	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no suitable tool found for %s in PATH (tried: %s)", envVar, strings.Join(candidates, ", "))
}

func isClang(compilerPath string) bool {
	base := strings.ToLower(filepath.Base(compilerPath))
	if strings.Contains(base, "clang") {
		return true
	}
	out, err := exec.Command(compilerPath, "--version").Output()
	return err == nil && strings.Contains(strings.ToLower(string(out)), "clang")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
