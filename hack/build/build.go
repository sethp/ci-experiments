package main

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/docker/buildx/util/progress"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	gateway "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/session/filesync"
	"github.com/opencontainers/go-digest"
	"github.com/spf13/pflag"
	fstypes "github.com/tonistiigi/fsutil/types"
	"github.com/yargevad/filepathx"
	"golang.org/x/sync/errgroup"

	_ "github.com/moby/buildkit/client/connhelper/dockercontainer"
	_ "github.com/sethp/ci-experiments/hack/build/connhelper/docker"
	"github.com/sethp/ci-experiments/hack/build/extcontext"
)

// go run ./hack/build should:
// 1. concurrently fire off builds against all of the leaf targets ("", test, lint, etc.)
// 2. that fail at the end (i.e. one interruption doesn't break the others)
// 3. producing separate and independent outputs for each
// 4. including (somehow? possibly?) flat stdio, especially on failure
//
// if this succeeds, I wonder what a handoff to a CI system would look like? point it to
// a git url, and it mounts that filesystem and... `go run ./hack/build` ? there's a lot there
// but also, do I want to make a whole 'nother language? Or use pseudo-data-as-code?

var (
	progressMode = "tty"
	connect      = "docker://"
	target       = "all"

	imageOpts = []llb.ImageOption{}
)

func init() {
	pflag.StringVar(&progressMode, "progress", progressMode, "plain or tty")
	pflag.StringVar(&connect, "connect", connect, "connection to buildkit (docker-container://<container> and docker[+<unix|tcp>]://[docker daemon] supported). Defaults to local docker daemon.")
	pflag.StringVar(&target, "target", target, "targets to build (all, lint, test, ...)")

	funczVar("pull", "force new pulls on images", func() error {
		imageOpts = append(imageOpts, llb.ResolveModeForcePull)
		return nil
	})
}

var (
	_, _ = fatal, slow
)

func test(ctx context.Context, c gateway.Client) (*llb.Definition, error) {
	return llb.
		Image("golang:alpine", append(imageOpts, []llb.ImageOption{
			metaResolver{c},
		}...)...).
		Dir("/go/src/github.com/sethp/ci-experiments").
		Run(
			llb.Shlex(`go test ./...`),
			llb.AddMount("/go/src/github.com/sethp/ci-experiments",
				llb.Local(".", llb.IncludePatterns(append(mustGlob("./**/*.go"), []string{"go.mod", "go.sum" /* TODO: testdata? others? */}...))),
				llb.Readonly,
			),
			llb.AddEnv("CGO_ENABLED", "0"),
			llb.AddEnv("GOOS", "linux"),
			llb.AddEnv("GOARCH", "amd64"),
			llb.AddMount("/root/.cache/go-build",
				llb.Scratch(),
				llb.AsPersistentCacheDir("/root/.cache/go-build", llb.CacheMountShared),
			),
			llb.AddMount("/go/pkg/mod",
				llb.Scratch(),
				llb.AsPersistentCacheDir("/go/pkg/mod", llb.CacheMountShared),
			),
		).
		Marshal(ctx)
}

func lint(ctx context.Context, c gateway.Client) (*llb.Definition, error) {
	return llb.
		Image("golang:alpine", append(imageOpts, []llb.ImageOption{
			metaResolver{c},
		}...)...).
		Dir("/go/src/github.com/sethp/ci-experiments").
		Run(
			llb.Args([]string{"golangci-lint", "run"}),
			llb.AddMount("/go/src/github.com/sethp/ci-experiments",
				llb.Local(".", llb.IncludePatterns(append(mustGlob("./**/*.go"), []string{"go.mod", "go.sum", ".golangci.yaml", ".golangci.yml", ".golangci.toml", ".golangci.json"}...))),
				llb.Readonly,
			),
			llb.AddEnv("CGO_ENABLED", "0"),
			llb.AddEnv("GOOS", "linux"),
			llb.AddEnv("GOARCH", "amd64"),
			llb.AddMount("/root/.cache/go-build",
				llb.Scratch(),
				llb.AsPersistentCacheDir("/root/.cache/go-build", llb.CacheMountShared),
			),
			llb.AddMount("/go/pkg/mod",
				llb.Scratch(),
				llb.AsPersistentCacheDir("/go/pkg/mod", llb.CacheMountShared),
			),
			llb.AddMount("/usr/bin/golangci-lint",
				llb.Image("golangci/golangci-lint:latest-alpine", imageOpts...),
				llb.SourcePath("/usr/bin/golangci-lint"),
				llb.Readonly,
			),
		).
		Marshal(ctx)
}

func fatal(ctx context.Context, c gateway.Client) (*llb.Definition, error) {
	return llb.Scratch().Run(llb.Args([]string{"always fails"})).Marshal(ctx)
}

func slow(ctx context.Context, c gateway.Client) (*llb.Definition, error) {
	return llb.Image("busybox").Run(llb.Args([]string{"sleep", "5"})).Marshal(ctx)
}

func main() {
	pflag.Parse()

	os.Exit(func() (exitcode int) {
		var (
			pw progress.Writer
		)

		ctx, cancel := extcontext.WithSignals(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		var (
			solveOpt client.SolveOpt
		)

		// TODO: shared session?
		solveOpt.Session = append(solveOpt.Session, filesync.NewFSSyncProvider([]filesync.SyncedDir{{
			Name: ".",
			Dir:  ".",
			Map: func(_ string, st *fstypes.Stat) bool {
				st.Uid = 0
				st.Gid = 0
				return true
			},
		}}))

		c, err := client.New(ctx, connect, client.WithFailFast())

		if err != nil {
			fmt.Fprintln(os.Stderr, "client.New() =", err)
			return 1
		}

		// Uses its own context to try and finish printing errors
		progressCtx, progressCancel := extcontext.WithGracePeriod(ctx, 500*time.Millisecond)
		defer progressCancel()
		pp := progress.NewPrinter(progressCtx, os.Stdout, progressMode)
		defer func() {
			if err := pp.Wait(); err != nil {
				fmt.Fprintln(os.Stderr, "progress.Wait() =", err)
				exitcode = 1
			}
		}()
		pw = pp

		var fn DefFunc
		switch target {
		case "all":
			// and now for something completely different

			// ignoring this returned context is what lets us run to the end even when one target fails
			// eg, _ := errgroup.WithContext(ctx)
			eg := errgroup.Group{}

			for _, dd := range []struct {
				name string
				fn   DefFunc
			}{{"lint", lint}, {"test", test}, {"fatal", fatal} /*{"async", async}*/} {
				pw := progress.WithPrefix(pw, dd.name, true /* this turns the prefix on or off? */)
				// pw = progress.ResetTime(pw)

				statusCh, progressDone := progress.NewChannel(pw)
				defer func() {
					<-progressDone
				}()
				fn := dd.fn
				eg.Go(func() error {
					_, err = c.Build(ctx, solveOpt, "TODO ???", BuildFunc(fn), statusCh)
					return err
				})
			}

			err := eg.Wait()
			if err != nil {
				fmt.Fprintln(os.Stderr, "eg.Wait() =", err)
				return 1
			}

			return
		case "lint":
			fn = lint
		case "test":
			fn = test
		case "fatal":
			fn = fatal
		case "slow":
			fn = slow
		default:
			fmt.Fprintf(os.Stderr, "unknown target: %q", target)
		}

		var (
			statusCh     chan *client.SolveStatus
			progressDone chan struct{}
			resp         *client.SolveResponse
		)
		pw = progress.ResetTime(pw)
		statusCh, progressDone = progress.NewChannel(pw)
		defer func() {
			<-progressDone
		}()

		resp, err = c.Build(ctx, solveOpt, "???", BuildFunc(fn), statusCh)
		if err != nil {
			<-progressDone
			fmt.Fprintln(os.Stderr, "c.Build(...) =", err)
			return 1
		}

		_ = resp
		// fmt.Printf("%#v", resp)

		return
	}())
}

type metaResolver struct {
	llb.ImageMetaResolver
}

func (m metaResolver) ResolveImageConfig(ctx context.Context, ref string, opt llb.ResolveImageConfigOpt) (digest.Digest, []byte, error) {
	return m.ImageMetaResolver.ResolveImageConfig(ctx, ref, llb.ResolveImageConfigOpt{
		Platform:    opt.Platform,
		ResolveMode: opt.ResolveMode,
		LogName:     fmt.Sprintf("[def] load metadata for %s", ref),
	})
}

func (m metaResolver) SetImageOption(ii *llb.ImageInfo) {
	llb.WithMetaResolver(m).SetImageOption(ii)
}

type DefFunc func(context.Context, gateway.Client) (*llb.Definition, error)

func BuildFunc(fn DefFunc) gateway.BuildFunc {
	return func(ctx context.Context, c gateway.Client) (*gateway.Result, error) {
		def, err := fn(ctx, c)
		if err != nil {
			return nil, err
		}
		return c.Solve(ctx, gateway.SolveRequest{
			Definition: def.ToPB(),
		})
	}
}

func mustGlob(pattern string) []string {
	paths, err := filepathx.Glob(pattern)
	if err != nil {
		panic(err)
	}
	return paths
}
