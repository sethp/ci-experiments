package main

import (
	"context"
	"fmt"
	"os"

	_ "github.com/moby/buildkit/client/connhelper/dockercontainer"
	_ "github.com/sethp/ci-experiments/hack/build/connhelper/docker"

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
	progressMode = pflag.String("progress", "tty", "plain or tty")
	connect      = pflag.String("connect", "docker://", "connection to buildkit (docker-container://<container> and docker[+<unix|tcp>]://[docker daemon] supported). Defaults to local docker daemon.")
)

func main() {
	pflag.Parse()

	ctx := context.Background() // TODO: watch for some signals?

	var def *llb.Definition

	def, err := llb.
		Image("golang:alpine", imagemetaresolver.WithDefault).
		Dir("/go/src/github.com/sethp/ci-experiments").
		Run(
			llb.Shlex(`go test .`),
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
		fmt.Fprintln(os.Stderr, `"llb.State".Marshal() =`, err)
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

	c, err := client.New(ctx, *connect, client.WithFailFast())
	if err != nil {
		fmt.Fprintln(os.Stderr, "client.New() =", err)
		os.Exit(1)
	}

	// mode := "tty"
	// mode := "plain"
	pp := progress.NewPrinter(context.Background(), os.Stdout, *progressMode)
	pw = pp

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

	resp, err = c.Build(ctx, solveOpt, "", f, statusCh)
	if err := pp.Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "progress.Wait() =", err)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "c.Build(...) =", err)
		os.Exit(1)
	}

	_ = resp
	// fmt.Printf("%#v", resp)
}
