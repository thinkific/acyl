package metahelm

import (
	"net/http"

	kubernetestrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	tokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

func DatadogTraceWrapTransport() func(rt http.RoundTripper) http.RoundTripper {
	ts := rest.NewCachedFileTokenSource(tokenFile)
	tokenWrappedTransport := rest.TokenSourceWrapTransport(ts)
	return func(rt http.RoundTripper) http.RoundTripper {
		return kubernetestrace.WrapRoundTripper(tokenWrappedTransport(rt))
	}
}
