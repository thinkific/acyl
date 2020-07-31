package secrets

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/dollarshaveclub/acyl/pkg/config"
	"github.com/dollarshaveclub/pvc"
	"github.com/pkg/errors"
)

// Secret IDs
// These will be interpolated as .ID in the secrets mapping
const (
	awsAccessKeyIDid           = "aws/access_key_id"
	awsSecretAccessKeyid       = "aws/secret_access_key"
	githubHookSecretid         = "github/hook_secret"
	githubTokenid              = "github/token"
	githubAppID                = "github/app/id"
	githubAppPK                = "github/app/private_key"
	githubAppHookSecret        = "github/app/hook_secret"
	githubOAuthInstID          = "github/app/oauth/installation_id"
	githubOAuthClientID        = "github/app/oauth/client/id"
	githubOAuthClientSecret    = "github/app/oauth/client/secret"
	githubOAuthCookieEncKey    = "github/app/oauth/cookie/encryption_key"
	githubOAuthCookieAuthKey   = "github/app/oauth/cookie/authentication_key"
	githubOAuthUserTokenEncKey = "github/app/oauth/user_token/encryption_key"
	apiKeysid                  = "api_keys"
	slackTokenid               = "slack/token"
	tlsCertid                  = "tls/cert"
	tlsKeyid                   = "tls/key"
	dbURIid                    = "db/uri"
)

type SecretFetcher interface {
	PopulateAllSecrets(aws *config.AWSCreds, gh *config.GithubConfig, slack *config.SlackConfig, srv *config.ServerConfig, pg *config.PGConfig) error
	PopulatePG(pg *config.PGConfig) error
	PopulateAWS(aws *config.AWSCreds) error
	PopulateGithub(gh *config.GithubConfig) error
	PopulateSlack(slack *config.SlackConfig) error
	PopulateServer(srv *config.ServerConfig) error
}

// PopulatePG will populate just the Postgres Configuration
// This is used in migration jobs that do not need all of the application secrets
func PopulatePG(secretsBackend string, secretsConfig *config.SecretsConfig, vaultConfig *config.VaultConfig, pgConfig *config.PGConfig) error {
	sf, err := newSecretFetcher(secretsBackend, secretsConfig, vaultConfig)
	if err != nil {
		return errors.Wrapf(err, "secrets.PopulatePG error creating new secret fetcher")
	}
	err = sf.PopulatePG(pgConfig)
	if err != nil {
		return errors.Wrapf(err, "secrets.PopulatePG error setting pgConfig")
	}
	return nil
}

// PopulateAllSecretsConfigurations is used to organize all the configs that need populating from the SecretsFetcher
type PopulateAllSecretsConfigurations struct {
	Backend       string
	SecretsConfig *config.SecretsConfig
	VaultConfig   *config.VaultConfig
	AWSCreds      *config.AWSCreds
	GithubConfig  *config.GithubConfig
	SlackConfig   *config.SlackConfig
	ServerConfig  *config.ServerConfig
	PGConfig      *config.PGConfig
}

// PopulateAllSecrets will take an PopulateAllSecretsConfigurations and will set each config based on the secrets backend and Vault method.
func PopulateAllSecrets(allConfigs *PopulateAllSecretsConfigurations) error {
	sf, err := newSecretFetcher(allConfigs.Backend, allConfigs.SecretsConfig, allConfigs.VaultConfig)
	if err != nil {
		return errors.Wrapf(err, "secrets.PopulatePG error creating new secret fetcher")
	}
	err = sf.PopulateAllSecrets(allConfigs.AWSCreds, allConfigs.GithubConfig, allConfigs.SlackConfig, allConfigs.ServerConfig, allConfigs.PGConfig)
	if err != nil {
		return errors.Wrapf(err, "secrets.PopulateAllSecrets error populating a configuration")
	}
	return nil
}

func newSecretFetcher(secretsBackend string, secretsConfig *config.SecretsConfig, vaultConfig *config.VaultConfig) (SecretFetcher, error) {
	if vaultConfig.UseAgent {
		sf := NewReadFileSecretsFetcher(vaultConfig)
		return sf, nil
	}
	ops := []pvc.SecretsClientOption{}
	switch secretsBackend {
	case "vault":
		secretsConfig.Backend = pvc.WithVaultBackend()
		switch {
		case vaultConfig.TokenAuth:
			log.Printf("secrets: using vault token auth")
			ops = []pvc.SecretsClientOption{
				pvc.WithVaultAuthentication(pvc.Token),
				pvc.WithVaultToken(vaultConfig.Token),
			}
		case vaultConfig.K8sAuth:
			log.Printf("secrets: using vault k8s auth")
			jwt, err := ioutil.ReadFile(vaultConfig.K8sJWTPath)
			if err != nil {
				errors.Wrapf(err, "error reading k8s jwt path: %v", err)
			}
			log.Printf("secrets: role: %v; auth path: %v", vaultConfig.K8sRole, vaultConfig.K8sAuthPath)
			ops = []pvc.SecretsClientOption{
				pvc.WithVaultAuthentication(pvc.K8s),
				pvc.WithVaultK8sAuth(string(jwt), vaultConfig.K8sRole),
				pvc.WithVaultK8sAuthPath(vaultConfig.K8sAuthPath),
			}
		case vaultConfig.AppID != "" && vaultConfig.UserIDPath != "":
			log.Printf("secrets: using vault AppID auth")
			ops = []pvc.SecretsClientOption{
				pvc.WithVaultAuthentication(pvc.AppID),
				pvc.WithVaultAppID(vaultConfig.AppID),
				pvc.WithVaultUserIDPath(vaultConfig.UserIDPath),
			}
		default:
			errors.New("no Vault authentication methods were supplied")
		}
		ops = append(ops, pvc.WithVaultHost(vaultConfig.Addr))
	case "env":
		secretsConfig.Backend = pvc.WithEnvVarBackend()
	default:
		errors.New(fmt.Sprintf("invalid secrets backend: %v", secretsBackend))
	}
	if secretsConfig.Mapping == "" {
		errors.New("secrets mapping is required")
	}
	ops = append(ops, pvc.WithMapping(secretsConfig.Mapping), secretsConfig.Backend)
	sc, err := pvc.NewSecretsClient(ops...)
	if err != nil {
		return nil, errors.Wrapf(err, "secrets.getSecretsClient error creating new PVC Secrets Client")
	}
	sf := NewPVCSecretsFetcher(sc)
	return sf, nil
}
