// +build linux darwin freebsd netbsd openbsd

package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"

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
	mockuiCmd.PersistentFlags().StringVar(&listenAddr, "listen-addr", "localhost:4000", "Listen address")
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
