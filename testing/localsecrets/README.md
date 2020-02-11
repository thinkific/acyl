# Local Secrets Injector

This tool injects working secrets into a running Acyl environment in a local k8s cluster (minikube, etc).

This is intended to allow full end-to-end tests in a local Acyl environment with real working secrets
such as GitHub API token, Slack API token, image pull secret, etc.

## Usage

```bash
$ GO111MODULE=off go build
$ export GITHUB_TOKEN=xxx
$ export SLACK_TOKEN=yyy # optional
$ kubectx minikube # make sure minikube is up and running, and there is a running Acyl DQA in it
$ ./localsecrets --image-pull-secret-path /tmp/image-pull-secret.json --dockercfg-path /tmp/dockercfg.json
```