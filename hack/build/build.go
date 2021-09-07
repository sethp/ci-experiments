package main

import (
	"context"
	"fmt"
	"os"

	_ "github.com/moby/buildkit/client/connhelper/dockercontainer"
	_ "github.com/sethp/ci-experiments/hack/build/connhelper/docker"
	"golang.org/x/sync/errgroup"

	"github.com/docker/buildx/util/progress"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/imagemetaresolver"
	gateway "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/session/filesync"
	"github.com/spf13/pflag"
	fstypes "github.com/tonistiigi/fsutil/types"
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
)

func init() {
	pflag.StringVar(&progressMode, "progress", progressMode, "plain or tty")
	pflag.StringVar(&connect, "connect", connect, "connection to buildkit (docker-container://<container> and docker[+<unix|tcp>]://[docker daemon] supported). Defaults to local docker daemon.")
	pflag.StringVar(&target, "target", target, "targets to build (all, lint, test, ...)")
}

func main() {
	pflag.Parse()

	ctx := context.Background() // TODO: watch for some signals?

	test, err := llb.
		Image("golang:alpine", imagemetaresolver.WithDefault).
		Dir("/go/src/github.com/sethp/ci-experiments").
		Run(
			llb.Shlex(`go test ./...`),
			llb.AddMount("/go/src/github.com/sethp/ci-experiments", llb.Local("."), llb.Readonly),
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
	if err != nil {
		fmt.Fprintln(os.Stderr, `test: "llb.State".Marshal() =`, err)
		os.Exit(1)
	}

	lint, err := llb.
		Image("golang:alpine", imagemetaresolver.WithDefault).
		Dir("/go/src/github.com/sethp/ci-experiments").
		Run(
			// llb.Shlex(`golangci-lint run`),
			llb.Args([]string{"golangci-lint", "run"}),
			llb.AddMount("/go/src/github.com/sethp/ci-experiments", llb.Local("."), llb.Readonly),
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
				llb.Image("golangci/golangci-lint:latest-alpine", imagemetaresolver.WithDefault),
				llb.SourcePath("/usr/bin/golangci-lint"),
				llb.Readonly,
			),
		).
		Marshal(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, `lint: "llb.State".Marshal() =`, err)
		os.Exit(1)
	}

	var (
		pw       progress.Writer
		f        gateway.BuildFunc
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
		os.Exit(1)
	}

	pp := progress.NewPrinter(context.Background(), os.Stdout, progressMode)
	defer func() {
		if err := pp.Wait(); err != nil {
			fmt.Fprintln(os.Stderr, "progress.Wait() =", err)
		}
	}()
	pw = pp
	fatal, err := llb.Scratch().Run(llb.Shlex("false")).Marshal(ctx)
	if err != nil {
		panic(err)
	}

	// where does this resolve from?
	// ahh! image meta gets resolved client-side at marshal time
	// async, err := llb.Scratch().Async(func(c1 context.Context, s llb.State, c2 *llb.Constraints) (llb.State, error) {
	// 	panic(s)
	// }).Marshal(ctx)
	// if err != nil {
	// 	panic(err)
	// }

	var def *llb.Definition
	switch target {
	case "all":
		// and now for something completely different

		// ignoring this returned context is what lets us run to the end even when one target fails
		// eg, _ := errgroup.WithContext(ctx)
		eg := errgroup.Group{}

		for _, dd := range []struct {
			name string
			def  *llb.Definition
		}{{"lint", lint}, {"test", test}, {"fatal", fatal} /*{"async", async}*/} {
			pw := progress.WithPrefix(pw, dd.name, true /* this turns the prefix on or off? */)

			statusCh, progressDone := progress.NewChannel(pw)
			defer func() {
				<-progressDone
			}()

			def := dd.def
			eg.Go(func() error {
				f = func(ctx context.Context, c gateway.Client) (*gateway.Result, error) {
					return c.Solve(ctx, gateway.SolveRequest{
						Definition: def.ToPB(),
					})
				}

				_, err = c.Build(ctx, solveOpt, "TODO ???", f, statusCh)
				return err
			})
		}

		err := eg.Wait()
		if err != nil {
			fmt.Fprintln(os.Stderr, "eg.Wait() =", err)
		}

		return
	case "lint":
		def = lint
	case "test":
		def = test
	default:
		panic(fmt.Sprintf("unknown target: %q", target))
	}

	f = func(ctx context.Context, c gateway.Client) (*gateway.Result, error) {
		return c.Solve(ctx, gateway.SolveRequest{
			Definition: def.ToPB(),
		})
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

	resp, err = c.Build(ctx, solveOpt, "???", f, statusCh)
	if err != nil {
		fmt.Fprintln(os.Stderr, "c.Build(...) =", err)
		os.Exit(1)
	}

	_ = resp
	// fmt.Printf("%#v", resp)
}
