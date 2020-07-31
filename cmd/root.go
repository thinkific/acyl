package cmd

import (
	"fmt"
	"os"

	"github.com/dollarshaveclub/acyl/pkg/config"
	"github.com/dollarshaveclub/acyl/pkg/secrets"
	"github.com/spf13/cobra"
)

// set by the main pkg
var (
	Version, Commit, Date string
)

var vaultConfig config.VaultConfig
var awsCreds config.AWSCreds
var awsConfig config.AWSConfig
var secretsConfig config.SecretsConfig
var secretsBackend string
var k8sClientConfig config.K8sClientConfig

var RootCmd = &cobra.Command{
	Use:   "acyl",
	Short: "Dynamic Environment System",
	Long:  `Dynamic environment server API and client utility`,
}

//shorthands in use by register: i p r m o
//shorthands here: abcdefg n jkl p q
func init() {
	RootCmd.PersistentFlags().StringVarP(&vaultConfig.Addr, "vault-addr", "a", os.Getenv("VAULT_ADDR"), "Vault URL (if using Vault secret backend)")
	RootCmd.PersistentFlags().BoolVar(&vaultConfig.UseAgent, "vault-agent", false, "Use Vault Agent Injector")
	RootCmd.PersistentFlags().StringVar(&vaultConfig.SecretsRootPath, "vault-secrets-root-path", "/vault/secrets", "Path where Vault Agent Injector is expected to write secrets ( if vault.UseAgent true )")
	RootCmd.PersistentFlags().StringVarP(&vaultConfig.Token, "vault-token", "b", os.Getenv("VAULT_TOKEN"), "Vault token (if using token auth & Vault secret backend)")
	RootCmd.PersistentFlags().BoolVarP(&vaultConfig.TokenAuth, "vault-token-auth", "c", false, "Use Vault token-based auth (if using Vault secret backend)")
	RootCmd.PersistentFlags().BoolVar(&vaultConfig.K8sAuth, "vault-k8s-auth", false, "Use Vault k8s auth (if using Vault secret backend)")
	RootCmd.PersistentFlags().StringVar(&vaultConfig.K8sJWTPath, "vault-k8s-jwt-path", "/var/run/secrets/kubernetes.io/serviceaccount/token", "Vault k8s JWT file path (if using k8s auth & Vault secret backend)")
	RootCmd.PersistentFlags().StringVar(&vaultConfig.K8sRole, "vault-k8s-role", "", "Vault k8s role (if using k8s auth & Vault secret backend)")
	RootCmd.PersistentFlags().StringVar(&vaultConfig.K8sAuthPath, "vault-k8s-auth-path", "kubernetes", "Vault k8s auth path (if using k8s auth & Vault secret backend)")
	RootCmd.PersistentFlags().StringVarP(&vaultConfig.AppID, "vault-app-id", "d", os.Getenv("APP_ID"), "Vault App-ID (if using Vault secret backend)")
	RootCmd.PersistentFlags().StringVarP(&vaultConfig.UserIDPath, "vault-user-id-path", "e", os.Getenv("USER_ID_PATH"), "Path to file containing Vault User-ID (if using Vault secret backend)")
	RootCmd.PersistentFlags().StringVarP(&awsConfig.Region, "aws-region", "k", "us-west-2", "AWS region")
	RootCmd.PersistentFlags().UintVarP(&awsConfig.MaxRetries, "aws-max-retries", "l", 3, "AWS max retries per operation")
	RootCmd.PersistentFlags().StringVar(&secretsBackend, "secrets-backend", "vault", "Secret backend (one of: vault,env)")
	RootCmd.PersistentFlags().StringVar(&secretsConfig.Mapping, "secrets-mapping", "", "Secrets mapping template string (required)")
	RootCmd.PersistentFlags().StringVar(&k8sClientConfig.JWTPath, "k8s-jwt-path", "/var/run/secrets/kubernetes.io/serviceaccount/token", "Path to the JWT used to authenticate the k8s client to the API server")
}

func clierr(msg string, params ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", params...)
	os.Exit(1)
}

func getSecrets() {
	err := secrets.PopulateAllSecrets(&secrets.PopulateAllSecretsConfigurations{
		Backend:       secretsBackend,
		SecretsConfig: &secretsConfig,
		VaultConfig:   &vaultConfig,
		AWSCreds:      &awsCreds,
		GithubConfig:  &githubConfig,
		SlackConfig:   &slackConfig,
		ServerConfig:  &serverConfig,
		PGConfig:      &pgConfig,
	})
	if err != nil {
		clierr("error getting secrets: %v", err)
	}
}

// Execute will execute the RootCmd
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
