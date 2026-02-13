//go:build integration

package itest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const cliTimeout = 30 * time.Second

type robustCase struct {
	name            string
	args            func(t *testing.T, repoRoot string) []string
	env             map[string]string
	wantContains    []string
	wantNotContains []string
}

type cliRunResult struct {
	exitCode int
	output   string
}

func TestRobustness_ArgsValidation(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	sample := filepath.Join(repoRoot, "internal", "itest", "testdata", "podcast_short.mp4")

	cases := []robustCase{
		{
			name: "no args",
			args: staticArgs(),
			wantContains: []string{
				"accepts 1 arg(s), received 0",
			},
		},
		{
			name: "too many args",
			args: staticArgs(sample, "extra"),
			wantContains: []string{
				"accepts 1 arg(s), received 2",
			},
		},
		{
			name: "unknown flag",
			args: staticArgs(sample, "--wat"),
			wantContains: []string{
				"unknown flag: --wat",
			},
		},
		{
			name: "clips non int",
			args: staticArgs(sample, "--clips", "nope"),
			wantContains: []string{
				`invalid argument "nope" for "--clips"`,
			},
		},
		{
			name: "clips zero",
			args: staticArgs(sample, "--clips", "0"),
			env: map[string]string{
				"OPENROUTER_API_KEY": "dummy",
			},
			wantContains: []string{
				"config: clips must be > 0",
			},
		},
		{
			name: "min flag removed",
			args: staticArgs(sample, "--min", "10"),
			wantContains: []string{
				"unknown flag: --min",
			},
		},
		{
			name: "max flag removed",
			args: staticArgs(sample, "--max", "120"),
			wantContains: []string{
				"unknown flag: --max",
			},
		},
	}

	runRobustCases(t, repoRoot, cases)
}

func TestRobustness_InvalidInputMedia(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	sample := filepath.Join(repoRoot, "internal", "itest", "testdata", "podcast_short.mp4")

	cases := []robustCase{
		{
			name: "missing input path",
			args: staticArgs(filepath.Join(repoRoot, "internal", "itest", "testdata", "does-not-exist.mp4")),
			env: map[string]string{
				"OPENROUTER_API_KEY": "dummy",
			},
			wantContains: []string{
				"config: stat input:",
			},
		},
		{
			name: "input is directory",
			args: staticArgs(filepath.Join(repoRoot, "internal", "itest", "testdata")),
			env: map[string]string{
				"OPENROUTER_API_KEY": "dummy",
			},
			wantContains: []string{
				"ffmpeg extract audio:",
			},
		},
		{
			name: "input is non media file",
			args: staticArgs(filepath.Join(repoRoot, "internal", "itest", "testdata", "not-media.txt")),
			env: map[string]string{
				"OPENROUTER_API_KEY": "dummy",
			},
			wantContains: []string{
				"ffmpeg extract audio:",
			},
		},
		{
			name: "out points to file",
			args: func(t *testing.T, _ string) []string {
				t.Helper()
				tmp := t.TempDir()
				outFile := filepath.Join(tmp, "out-file")
				if err := os.WriteFile(outFile, []byte("x"), 0o644); err != nil {
					t.Fatalf("write out file fixture: %v", err)
				}
				return []string{sample, "--out", outFile}
			},
			env: map[string]string{
				"OPENROUTER_API_KEY": "dummy",
			},
			wantContains: []string{
				"not a directory",
			},
		},
	}

	runRobustCases(t, repoRoot, cases)
}

func TestRobustness_SecurityEnvHardening(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	sample := filepath.Join(repoRoot, "internal", "itest", "testdata", "podcast_short.mp4")

	cases := []robustCase{
		{
			name: "reject base url with http",
			args: staticArgs(sample),
			env: map[string]string{
				"OPENROUTER_API_KEY":  "dummy",
				"OPENROUTER_BASE_URL": "http://openrouter.ai",
			},
			wantContains: []string{
				"https is required",
			},
		},
		{
			name: "reject base url unknown host",
			args: staticArgs(sample),
			env: map[string]string{
				"OPENROUTER_API_KEY":  "dummy",
				"OPENROUTER_BASE_URL": "https://evil.example",
			},
			wantContains: []string{
				`is not in OPENROUTER_ALLOWED_HOSTS`,
			},
		},
		{
			name: "reject base url userinfo",
			args: staticArgs(sample),
			env: map[string]string{
				"OPENROUTER_API_KEY":  "dummy",
				"OPENROUTER_BASE_URL": "https://user:pass@openrouter.ai",
			},
			wantContains: []string{
				"userinfo is not allowed",
			},
		},
		{
			name: "reject base url query and fragment",
			args: staticArgs(sample),
			env: map[string]string{
				"OPENROUTER_API_KEY":  "dummy",
				"OPENROUTER_BASE_URL": "https://openrouter.ai?x=1",
			},
			wantContains: []string{
				"query and fragment are not allowed",
			},
		},
		{
			name: "reject empty api key",
			args: staticArgs(sample),
			env: map[string]string{
				"OPENROUTER_API_KEY": "",
			},
			wantContains: []string{
				"OPENROUTER_API_KEY is required",
			},
		},
		{
			name: "allow configured base url host then fail later",
			args: staticArgs(filepath.Join(repoRoot, "internal", "itest", "testdata")),
			env: map[string]string{
				"OPENROUTER_API_KEY":       "dummy",
				"OPENROUTER_BASE_URL":      "https://proxy.internal",
				"OPENROUTER_ALLOWED_HOSTS": " proxy.internal ",
			},
			wantContains: []string{
				"ffmpeg extract audio:",
			},
			wantNotContains: []string{
				"invalid OPENROUTER_BASE_URL",
			},
		},
	}

	runRobustCases(t, repoRoot, cases)
}

func runRobustCases(t *testing.T, repoRoot string, cases []robustCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := runCLI(t, repoRoot, tc.args(t, repoRoot), tc.env)
			if res.exitCode == 0 {
				t.Fatalf("expected non-zero exit code, got 0\noutput:\n%s", res.output)
			}
			for _, want := range tc.wantContains {
				if !strings.Contains(res.output, want) {
					t.Fatalf("expected output to contain %q\noutput:\n%s", want, res.output)
				}
			}
			for _, notWant := range tc.wantNotContains {
				if strings.Contains(res.output, notWant) {
					t.Fatalf("expected output to not contain %q\noutput:\n%s", notWant, res.output)
				}
			}
		})
	}
}

func runCLI(t *testing.T, repoRoot string, args []string, env map[string]string) cliRunResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), cliTimeout)
	defer cancel()

	cmdArgs := append([]string{"run", "./cmd/hlcut"}, args...)
	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Dir = repoRoot
	cmd.Env = mergeEnv(
		os.Environ(),
		map[string]string{
			"NO_COLOR": "1",
			"TERM":     "dumb",
		},
		env,
	)

	out, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("command timed out after %s: go %s", cliTimeout, strings.Join(cmdArgs, " "))
	}

	res := cliRunResult{output: string(out)}
	if err == nil {
		res.exitCode = 0
		return res
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.exitCode = exitErr.ExitCode()
		return res
	}

	t.Fatalf("run command: %v\noutput:\n%s", err, string(out))
	return cliRunResult{}
}

func mergeEnv(base []string, overrides ...map[string]string) []string {
	env := make(map[string]string, len(base))
	for _, kv := range base {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			continue
		}
		env[kv[:i]] = kv[i+1:]
	}

	for _, set := range overrides {
		for k, v := range set {
			env[k] = v
		}
	}

	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(out)
	return out
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()

	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	return repoRoot
}

func staticArgs(args ...string) func(t *testing.T, _ string) []string {
	clone := append([]string(nil), args...)
	return func(t *testing.T, _ string) []string {
		t.Helper()
		return append([]string(nil), clone...)
	}
}
