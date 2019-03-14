package images

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/dollarshaveclub/acyl/pkg/eventlogger"
	"github.com/dollarshaveclub/acyl/pkg/ghclient"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/dollarshaveclub/acyl/pkg/persistence"
	"github.com/mholt/archiver"
	"github.com/pkg/errors"
)

type DockerClient interface {
	ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error)
}

// DockerBuilderBackend builds images using a Docker Engine
type DockerBuilderBackend struct {
	DC    DockerClient
	RC    ghclient.RepoClient
	DL    persistence.DataLayer
	Auths map[string]types.AuthConfig
	Push  bool
}

var _ BuilderBackend = &DockerBuilderBackend{}

func (dbb *DockerBuilderBackend) log(ctx context.Context, msg string, args ...interface{}) {
	eventlogger.GetLogger(ctx).Printf("docker builder: "+msg, args...)
}

// BuildImage synchronously builds and optionally pushes the image using the Docker Engine, returning when the build completes.
func (dbb *DockerBuilderBackend) BuildImage(ctx context.Context, envName, githubRepo, imageRepo, ref string, ops BuildOptions) error {
	if dbb.DC == nil {
		return errors.New("docker client is nil")
	}
	if dbb.DL == nil {
		return errors.New("datalayer is nil")
	}
	if dbb.RC == nil {
		return errors.New("repo client is nil")
	}
	if ops.DockerfilePath == "" {
		ops.DockerfilePath = "Dockerfile"
	}
	tdir, err := ioutil.TempDir("", "acyl-docker-builder")
	if err != nil {
		return errors.Wrap(err, "error getting temp dir")
	}
	defer os.RemoveAll(tdir)
	dbb.log(ctx, "building context tar for %v", githubRepo)
	contents, err := dbb.RC.GetDirectoryContents(ctx, githubRepo, "", ref)
	if err != nil {
		return errors.Wrap(err, "error getting repo directory contents")
	}
	for file, content := range contents {
		dir := filepath.Join(tdir, filepath.Dir(file))
		if err := os.MkdirAll(dir, os.ModeDir|os.ModePerm); err != nil {
			return errors.Wrap(err, "error creating file directory")
		}
		fp := filepath.Join(tdir, file)
		if content.Symlink {
			if err := os.Symlink(content.SymlinkTarget, fp); err != nil {
				return errors.Wrap(err, "error creating symlink")
			}
			continue
		}
		if err := ioutil.WriteFile(fp, content.Contents, os.ModePerm); err != nil {
			return errors.Wrap(err, "error writing file")
		}
	}
	bctx, err := ioutil.TempFile("", "acyl-docker-builder-context-*.tar")
	if err != nil {
		return errors.Wrap(err, "error creating tar temp file")
	}
	bctx.Close()
	os.Remove(bctx.Name())
	f, err := os.Open(tdir)
	if err != nil {
		return errors.Wrap(err, "error opening temp dir")
	}
	entries, err := f.Readdir(0)
	if err != nil {
		return errors.Wrap(err, "error reading temp dir names")
	}
	files := make([]string, len(entries))
	for i := range entries {
		files[i] = filepath.Join(tdir, entries[i].Name())
	}
	tar := archiver.NewTar()
	tar.ContinueOnError = true // ignore things like broken symlinks
	if err := tar.Archive(files, bctx.Name()); err != nil {
		return errors.Wrap(err, "error writing tar file")
	}
	defer os.Remove(bctx.Name())
	f, err = os.Open(bctx.Name())
	if err != nil {
		return errors.Wrap(err, "error opening tar")
	}
	defer f.Close()
	bargs := make(map[string]*string, len(ops.BuildArgs))
	for k, v := range ops.BuildArgs {
		v := v
		bargs[k] = &v
	}
	opts := types.ImageBuildOptions{
		Tags:        []string{imageRepo + ":" + ref},
		Remove:      true,
		ForceRemove: true,
		PullParent:  true,
		Dockerfile:  ops.DockerfilePath,
		BuildArgs:   bargs,
		AuthConfigs: dbb.Auths,
	}
	span, _ := tracer.SpanFromContext(ctx)
	dbb.DL.AddEvent(span, envName, fmt.Sprintf("building container: %v:%v", githubRepo, ref))
	dbb.log(ctx, "building image: %v", opts.Tags[0])
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for _ = range ticker.C {
			dbb.log(ctx, "... still building %v:%v", githubRepo, ref)
		}
	}()
	resp, err := dbb.DC.ImageBuild(ctx, f, opts)
	ticker.Stop()
	if err != nil {
		return errors.Wrap(err, "error starting image build")
	}
	err = handleOutput(resp.Body)
	if err != nil {
		return errors.Wrap(err, "error performing build")
	}
	if dbb.Push {
		rsl := strings.Split(imageRepo, "/")
		var registryURLs []string
		switch len(rsl) {
		case 2: // Docker Hub
			registryURLs = []string{"https://index.docker.io/v1/", "https://index.docker.io/v2/"}
		case 3: // private registry
			registryURLs = []string{"https://" + rsl[0]}
		default:
			return fmt.Errorf("cannot determine base registry URL from %v", imageRepo)
		}
		var auth string
		for _, url := range registryURLs {
			val, ok := dbb.Auths[url]
			if ok {
				j, err := json.Marshal(&val)
				if err != nil {
					return fmt.Errorf("error marshaling auth: %v", err)
				}
				auth = base64.StdEncoding.EncodeToString(j)
			}
		}
		if auth == "" {
			return fmt.Errorf("auth not found for %v", imageRepo)
		}
		opts := types.ImagePushOptions{
			All:          true,
			RegistryAuth: auth,
		}
		dbb.log(ctx, "pushing image: %v", imageRepo+":"+ref)
		ticker = time.NewTicker(5 * time.Second)
		go func() {
			for _ = range ticker.C {
				dbb.log(ctx, "... still pushing %v:%v", githubRepo, ref)
			}
		}()
		resp, err := dbb.DC.ImagePush(ctx, imageRepo+":"+ref, opts)
		ticker.Stop()
		if err != nil {
			return errors.Wrap(err, "error starting image push")
		}
		err = handleOutput(resp)
		if err != nil {
			return errors.Wrap(err, "error pushing image")
		}
		dbb.log(ctx, "image pushed: %v", imageRepo+":"+ref)
	}
	return nil
}

func handleOutput(resp io.ReadCloser) error {
	defer resp.Close()
	return jsonmessage.DisplayJSONMessagesStream(resp, ioutil.Discard, 0, false, nil)
}
