// Package docker provides connhelper for docker://[default|container]
//
// If no container is provided, or the container name is "default", the connection
// will be to the built-in buildkit running inside the docker daemon.
//
// Otherwise, this connhelper functions like "github.com/moby/buildkit/client/connhelper/dockercontainer"
package docker

import (
	"net/url"
	"testing"
)

func Test_hostFromUrl(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{
			url:  "docker://",
			want: "",
		},
		{
			url:  "docker:///var/run/docker.sock",
			want: "unix:///var/run/docker.sock",
		},
		{
			url:  "docker://dockerd:2375",
			want: "tcp://dockerd:2375",
		},
		{
			url:  "docker://var/run/mysock",
			want: "tcp://var/run/mysock", // descriptive
		},
		{
			url:  "docker+unix://docker.sock",
			want: "unix://docker.sock",
		},
		{
			url:  "docker+unix://run/docker.sock",
			want: "unix://run/docker.sock",
		},
		{
			url:  "docker+tcp://dockerd:2375/basepath",
			want: "tcp://dockerd:2375/basepath",
		},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			arg, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("invalid arg `%s`: %v", tt.url, err)
			}
			if got := hostFromUrl(arg); got != tt.want {
				t.Errorf("hostFromUrl() = %v, want %v", got, tt.want)
			}
		})
	}
}
