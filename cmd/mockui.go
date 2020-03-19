// +build linux darwin freebsd netbsd openbsd

package cmd

import (
	"log"
	"net/http"
	"os"

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

var apiBaseURL, assetsPath, routePrefix, listenAddr string

func init() {
	mockuiCmd.PersistentFlags().StringVar(&apiBaseURL, "base-url", "http://localhost:4000", "API base URL")
	mockuiCmd.PersistentFlags().StringVar(&assetsPath, "assets-path", "ui/", "local filesystem path for UI assets")
	mockuiCmd.PersistentFlags().StringVar(&routePrefix, "route-prefix", "/ui", "UI base URL prefix")
	mockuiCmd.PersistentFlags().StringVar(&listenAddr, "listen-addr", "0.0.0.0:4000", "Listen address")
	RootCmd.AddCommand(mockuiCmd)
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

	if err := httpapi.RegisterVersions(deps, api.WithUIBaseURL(apiBaseURL), api.WithUIAssetsPath(assetsPath), api.WithUIRoutePrefix(routePrefix), api.WithUIReload()); err != nil {
		log.Fatalf("error registering api versions: %v", err)
	}

	logger.Printf("listening on: %v", listenAddr)
	logger.Println(server.ListenAndServe())
}
