// +build linux darwin freebsd netbsd openbsd

package cmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dollarshaveclub/acyl/pkg/nitro/metahelm"
	"github.com/dollarshaveclub/acyl/pkg/spawner"
	"github.com/google/uuid"

	"github.com/dollarshaveclub/acyl/pkg/config"
	"github.com/dollarshaveclub/acyl/pkg/ghclient"
	"github.com/dollarshaveclub/acyl/pkg/models"
	"github.com/dollarshaveclub/acyl/pkg/persistence"

	"github.com/spf13/cobra"

	"github.com/dollarshaveclub/acyl/pkg/api"
)

// serverCmd represents the server command
var mockuiCmd = &cobra.Command{
	Use:   "mockui",
	Short: "Run a mock UI server",
	Long:  `Run a mock UI HTTP server for UI development/testing`,
	Run:   mockui,
}

var listenAddr, mockDataFile, mockUser string
var mockRepos []string
var readOnly bool

func addUIFlags(cmd *cobra.Command) {
	brj, err := json.Marshal(&config.DefaultUIBranding)
	if err != nil {
		log.Fatalf("error marshaling default UI branding: %v", err)
	}
	cmd.PersistentFlags().StringVar(&serverConfig.UIBaseURL, "ui-base-url", "", "External base URL (https://somedomain.com) for UI links")
	cmd.PersistentFlags().StringVar(&serverConfig.UIPath, "ui-path", "/opt/ui", "Local filesystem path to UI assets")
	cmd.PersistentFlags().StringVar(&serverConfig.UIBaseRoute, "ui-base-route", "/ui", "Base prefix for UI HTTP routes")
	cmd.PersistentFlags().StringVar(&serverConfig.UIBrandingJSON, "ui-branding", string(brj), "Branding JSON configuration (see doc)")
	cmd.PersistentFlags().StringSliceVar(&serverConfig.UIExternalDocumentURLs, "mock-doc-urls", []string{}, "External documentation URLs")
	cmd.PersistentFlags().BoolVar(&githubConfig.OAuth.Enforce, "ui-enforce-oauth", false, "Enforce GitHub App OAuth authn/authz for UI routes")
	cmd.PersistentFlags().StringVar(&mockDataFile, "mock-data", "testdata/data.json", "Path to mock data file")
	cmd.PersistentFlags().StringVar(&mockUser, "mock-user", "bobsmith", "Mock username (for sessions)")
	cmd.PersistentFlags().StringSliceVar(&mockRepos, "mock-repos", []string{"acme/microservice", "acme/widgets", "acme/customers"}, "Mock repo read write permissions (for session user)")
	cmd.PersistentFlags().BoolVar(&readOnly, "mock-read-only", false, "Mock repo override to read only permissions (for session user)")
}

func init() {
	mockuiCmd.PersistentFlags().StringVar(&listenAddr, "listen-addr", "localhost:4000", "Listen address")
	addUIFlags(mockuiCmd)
	RootCmd.AddCommand(mockuiCmd)
}

func mockEvents(fdl *persistence.FakeDataLayer, qae []models.QAEnvironment) {
	// add some mock events
	if len(qae) == 0 {
		return
	}
	env := qae[0]
	id := fdl.NewFakeEvent(env.Created.Add(1*time.Hour), env.Repo, env.User, env.Name, models.UpdateEvent, true)
	log.Printf("creating fake update event for %v: %v", env.Name, id)
	id = fdl.NewFakeEvent(env.Created.Add(2*time.Hour), env.Repo, env.User, env.Name, models.UpdateEvent, false)
	log.Printf("creating fake update event for %v (failure): %v", env.Name, id)
	id = fdl.NewFakeEvent(env.Created.Add(3*time.Hour), env.Repo, env.User, env.Name, models.DestroyEvent, true)
	log.Printf("creating fake destroy event for %v: %v", env.Name, id)
}

func loadMockData(fpath string) *persistence.FakeDataLayer {
	f, err := os.Open(fpath)
	if err != nil {
		log.Fatalf("error opening mock data file: %v", err)
	}
	defer f.Close()
	td := testData{}
	if err := json.NewDecoder(f).Decode(&td); err != nil {
		log.Fatalf("error unmarshaling mock data file: %v", err)
	}
	now := time.Now().UTC()
	for i := range td.QAEnvironments {
		td.QAEnvironments[i].Created = now.AddDate(0, 0, -(i * 3))
	}
	for i := range td.K8sEnvironments {
		td.K8sEnvironments[i].Created = now.AddDate(0, 0, -(i * 3))
		td.K8sEnvironments[i].Updated.Time = td.K8sEnvironments[i].Created.Add(1 * time.Hour)
		td.K8sEnvironments[i].Updated.Valid = true
	}
	for i := range td.HelmReleases {
		td.HelmReleases[i].Created = now.AddDate(0, 0, -(i * 3))
	}
	fdl := persistence.NewPopulatedFakeDataLayer(td.QAEnvironments, td.K8sEnvironments, td.HelmReleases)
	for _, qae := range td.QAEnvironments {
		log.Printf("creating fake create event for env: %v, repo: %v, user: %v: %v", qae.Name, qae.Repo, qae.User, fdl.NewFakeCreateEvent(qae.Created, qae.Repo, qae.User, qae.Name))
	}
	mockEvents(fdl, td.QAEnvironments)
	return fdl
}

// randomPEMKey generates a random RSA key in PEM format
func randomPEMKey() []byte {
	reader := rand.Reader
	key, err := rsa.GenerateKey(reader, 2048)
	if err != nil {
		log.Fatalf("error generating random PEM key: %v", err)
	}

	var privateKey = &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	out := &bytes.Buffer{}
	if err := pem.Encode(out, privateKey); err != nil {
		log.Fatalf("error encoding PEM key: %v", err)
	}
	return out.Bytes()
}

func setDummyGHConfig() {
	githubConfig.OAuth.Enforce = true // using dummy session user
	githubConfig.PrivateKeyPEM = randomPEMKey()
	githubConfig.AppID = 1
	githubConfig.AppHookSecret = "asdf"
	copy(githubConfig.OAuth.UserTokenEncKey[:], []byte("00000000000000000000000000000000"))
}

func mockui(cmd *cobra.Command, args []string) {

	logger := log.New(os.Stderr, "", log.LstdFlags)

	server := &http.Server{Addr: listenAddr}

	httpapi := api.NewDispatcher(server)
	var dl *persistence.FakeDataLayer
	if mockDataFile != "" {
		dl = loadMockData(mockDataFile)
	} else {
		dl = persistence.NewFakeDataLayer()
	}
	dl.CreateMissingEventLog = true
	uf := func(ctx context.Context, rd models.RepoRevisionData) (string, error) {
		return "updated environment", nil
	}
	deps := &api.Dependencies{
		DataLayer:    dl,
		ServerConfig: serverConfig,
		Logger:       logger,
		EnvironmentSpawner: &spawner.FakeEnvironmentSpawner{UpdateFunc: uf},
		KubernetesReporter: metahelm.FakeKubernetesReporter{FakePodLogFilePath: "pkg/nitro/metahelm/testdata/pod_logs.log"},
	}

	serverConfig.UIBaseURL = "http://" + listenAddr

	var branding config.UIBrandingConfig
	if err := json.Unmarshal([]byte(serverConfig.UIBrandingJSON), &branding); err != nil {
		log.Fatalf("error unmarshaling branding config: %v", err)
	}

	setDummyGHConfig()

	httpapi.AppGHClientFactoryFunc = func(_ string) ghclient.GitHubAppInstallationClient {
		return &ghclient.FakeRepoClient{
			GetUserAppRepoPermissionsFunc: func(_ context.Context, instID int64) (map[string]ghclient.AppRepoPermissions, error) {
				out := make(map[string]ghclient.AppRepoPermissions, len(mockRepos))
				for _, r := range mockRepos {
					if readOnly {
						out[r] = ghclient.AppRepoPermissions{
							Repo: r,
							Pull: true,
						}
					} else {
						out[r] = ghclient.AppRepoPermissions{
							Repo: r,
							Pull: true,
							Push: true,
						}
					}
				}
				return out, nil
			},
		}
	}

	if err := httpapi.RegisterVersions(deps,
		api.WithGitHubConfig(githubConfig),
		api.WithUIBaseURL(serverConfig.UIBaseURL),
		api.WithUIAssetsPath(serverConfig.UIPath),
		api.WithUIRoutePrefix(serverConfig.UIBaseRoute),
		api.WithUIReload(),
		api.WithUIBranding(branding),
		api.WithUIDummySessionUser(mockUser)); err != nil {
		log.Fatalf("error registering api versions: %v", err)
	}

	go func() {
		logger.Printf("listening on: %v", listenAddr)
		logger.Println(server.ListenAndServe())
	}()

	opencmd := fmt.Sprintf("%v http://%v/ui/event/status?id=%v", openPath, listenAddr, uuid.Must(uuid.NewRandom()))
	shellsl := strings.Split(shell, " ")
	cmdsl := append(shellsl, opencmd)
	c := exec.Command(cmdsl[0], cmdsl[1:]...)
	if out, err := c.CombinedOutput(); err != nil {
		log.Fatalf("error opening UI browser: %v: %v: %v", strings.Join(cmdsl, " "), string(out), err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	output.Green().Progress().Println("Keeping UI server running (ctrl-c to exit)...")
	<-done
	if server != nil {
		server.Shutdown(context.Background())
	}
}
