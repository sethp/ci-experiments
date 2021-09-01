// Package docker provides connhelper for docker://[default|container]
//
// If no container is provided, or the container name is "default", the connection
// will be to the built-in buildkit running inside the docker daemon.
//
// Otherwise, this connhelper functions like "github.com/moby/buildkit/client/connhelper/dockercontainer"
package docker

import (
	"context"
	"net"
	"net/url"
	"strings"

	dockerclient "github.com/docker/docker/client"
	"github.com/moby/buildkit/client/connhelper"
)

func init() {
	connhelper.Register("docker", Helper)
	connhelper.Register("docker+unix", Helper)
	connhelper.Register("docker+tcp", Helper)
}

// Helper returns helper for connecting to a Docker daemon.
// This uses the buildkit built into the docker daemon since version 18.09.
func Helper(u *url.URL) (*connhelper.ConnectionHelper, error) {
	opts := []dockerclient.Opt{dockerclient.FromEnv}
	if host := hostFromUrl(u); host != "" {
		opts = append(opts, dockerclient.WithHost(host))
	}
	d, err := dockerclient.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	return &connhelper.ConnectionHelper{
		ContextDialer: func(ctx context.Context, addr string) (net.Conn, error) {
			return d.DialHijack(ctx, "/grpc", "h2c", nil)
		},
	}, nil
}

// URL is like docker[+<unix|tcp>]://[target]
// We'll mishandle relative paths for unix sockets unless the protocol is explicitly docker+unix://
func hostFromUrl(u *url.URL) string {
	parts := strings.SplitN(u.Scheme, "+", 2)
	if len(parts) == 1 && len(u.Host) == 0 && len(u.Path) == 0 {
		// `docker://`
		return ""
	}
	var scheme string
	if len(parts) == 1 {
		if len(u.Host) > 0 {
			scheme = "tcp://"
		} else {
			scheme = "unix://"
		}
	} else {
		scheme = parts[1] + "://"
	}

	return strings.Join([]string{scheme, u.Host, u.Path}, "")
}
