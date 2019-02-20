package images

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"testing"

	"github.com/dollarshaveclub/acyl/pkg/ghclient"
	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/pkg/errors"

	"github.com/docker/docker/api/types"
)

type fakeDockerClient struct {
	ImageBuildFunc func(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	ImagePushFunc  func(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error)
}

func (fdc *fakeDockerClient) ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error) {
	if fdc.ImageBuildFunc != nil {
		return fdc.ImageBuildFunc(ctx, buildContext, options)
	}
	return types.ImageBuildResponse{Body: ioutil.NopCloser(&bytes.Buffer{})}, nil
}
func (fdc *fakeDockerClient) ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error) {
	if fdc.ImagePushFunc != nil {
		return fdc.ImagePushFunc(ctx, image, options)
	}
	return ioutil.NopCloser(&bytes.Buffer{}), nil
}

func TestDockerBackendBuild(t *testing.T) {
	rc := &ghclient.FakeRepoClient{
		GetDirectoryContentsFunc: func(ctx context.Context, repo, path, ref string) (map[string]ghclient.FileContents, error) {
			return map[string]ghclient.FileContents{
				"foo/bar.txt": ghclient.FileContents{
					Path:     "foo/bar.txt",
					Contents: []byte("asdf"),
				},
				"foo/123.txt": ghclient.FileContents{
					Path:     "foo/123.txt",
					Contents: []byte("asdf123"),
				},
				"biz/123/zxcv/1.txt": ghclient.FileContents{
					Path:     "biz/123/zxcv/1.txt",
					Contents: []byte("asdf"),
				},
			}, nil
		},
	}
	var builderr, pusherr bool
	var built, pushed bool
	dbb := DockerBuilderBackend{
		DC: &fakeDockerClient{
			ImageBuildFunc: func(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error) {
				built = true
				if builderr {
					return types.ImageBuildResponse{}, errors.New("build failure")
				}
				return types.ImageBuildResponse{Body: ioutil.NopCloser(&bytes.Buffer{})}, nil
			},
			ImagePushFunc: func(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error) {
				pushed = true
				if pusherr {
					return nil, errors.New("push failure")
				}
				return ioutil.NopCloser(&bytes.Buffer{}), nil
			},
		},
		DL: persistence.NewFakeDataLayer(),
		RC: rc,
		Auths: map[string]types.AuthConfig{
			"https://quay.io": types.AuthConfig{},
		},
		Push: false,
	}
	err := dbb.BuildImage(context.Background(), "some-name", "acme/widgets", "quay.io/acme/widgets", "asdf", BuildOptions{})
	if err != nil {
		t.Fatalf("build should have succeeded: %v", err)
	}
	if !built {
		t.Fatalf("ImageBuild should have been called")
	}
	if pushed {
		t.Fatalf("ImagePush shouldn't have been called")
	}
	builderr = true
	err = dbb.BuildImage(context.Background(), "some-name", "acme/widgets", "quay.io/acme/widgets", "asdf", BuildOptions{})
	if err == nil {
		t.Fatalf("build should have failed")
	}
	built = false
	builderr = false
	dbb.Push = true
	err = dbb.BuildImage(context.Background(), "some-name", "acme/widgets", "quay.io/acme/widgets", "asdf", BuildOptions{})
	if err != nil {
		t.Fatalf("build should have succeeded: %v", err)
	}
	if !built {
		t.Fatalf("ImageBuild should have been called")
	}
	if !pushed {
		t.Fatalf("ImagePush should have have been called")
	}
	pusherr = true
	err = dbb.BuildImage(context.Background(), "some-name", "acme/widgets", "quay.io/acme/widgets", "asdf", BuildOptions{})
	if err == nil {
		t.Fatalf("build should have failed")
	}
	err = dbb.BuildImage(context.Background(), "some-name", "acme/widgets", "privateregistry.io/acme/widgets", "asdf", BuildOptions{})
	if err == nil {
		t.Fatalf("build should have failed with missing auth")
	}
}
