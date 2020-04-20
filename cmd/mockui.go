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

	"github.com/google/uuid"

	"github.com/dollarshaveclub/acyl/pkg/config"
	"github.com/dollarshaveclub/acyl/pkg/persistence"

	"github.com/dollarshaveclub/acyl/pkg/api"
	"github.com/spf13/cobra"
)

// serverCmd represents the server command
var mockuiCmd = &cobra.Command{
	Use:   "mockui",
	Short: "Run a mock UI server",
	Long:  `Run a mock UI HTTP server for UI development/testing`,
	Run:   mockui,
}

var listenAddr string

func addUIFlags(cmd *cobra.Command) {
	brj, err := json.Marshal(&config.DefaultUIBranding)
	if err != nil {
		log.Fatalf("error marshaling default UI branding: %v", err)
	}
	cmd.PersistentFlags().StringVar(&serverConfig.UIBaseURL, "ui-base-url", "", "External base URL (https://somedomain.com) for UI links")
	cmd.PersistentFlags().StringVar(&serverConfig.UIPath, "ui-path", "/opt/ui", "Local filesystem path to UI assets")
	cmd.PersistentFlags().StringVar(&serverConfig.UIBaseRoute, "ui-base-route", "/ui", "Base prefix for UI HTTP routes")
	cmd.PersistentFlags().StringVar(&serverConfig.UIBrandingJSON, "ui-branding", string(brj), "Branding JSON configuration (see doc)")
}

func init() {
	mockuiCmd.PersistentFlags().StringVar(&listenAddr, "listen-addr", "localhost:4000", "Listen address")
	addUIFlags(mockuiCmd)
	RootCmd.AddCommand(mockuiCmd)
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

func mockui(cmd *cobra.Command, args []string) {

	logger := log.New(os.Stderr, "", log.LstdFlags)

	server := &http.Server{Addr: listenAddr}

	httpapi := api.NewDispatcher(server)
	dl := persistence.NewFakeDataLayer()
	dl.CreateMissingEventLog = true
	deps := &api.Dependencies{
		DataLayer:    dl,
		ServerConfig: serverConfig,
		Logger:       logger,
	}

	serverConfig.UIBaseURL = "http://" + listenAddr

	var branding config.UIBrandingConfig
	if err := json.Unmarshal([]byte(serverConfig.UIBrandingJSON), &branding); err != nil {
		log.Fatalf("error unmarshaling branding config: %v", err)
	}

	// dummy values
	githubConfig.PrivateKeyPEM = randomPEMKey()
	githubConfig.AppID = 1
	githubConfig.AppHookSecret = "asdf"

	if err := httpapi.RegisterVersions(deps,
		api.WithGitHubConfig(githubConfig),
		api.WithUIBaseURL(serverConfig.UIBaseURL),
		api.WithUIAssetsPath(serverConfig.UIPath),
		api.WithUIRoutePrefix(serverConfig.UIBaseRoute),
		api.WithUIReload(),
		api.WithUIBranding(branding)); err != nil {
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
