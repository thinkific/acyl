package secrets

import (
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"
	"io/ioutil"
	"github.com/dollarshaveclub/acyl/pkg/config"

	"github.com/pkg/errors"
)
var vaultSecretsRootPath = "/vault/secrets/"

type ReadFileSecretsFetcher struct {}

// PopulateAllSecrets populates all secrets into the respective config structs
func (rf *ReadFileSecretsFetcher) PopulateAllSecrets(aws *config.AWSCreds, gh *config.GithubConfig, slack *config.SlackConfig, srv *config.ServerConfig, pg *config.PGConfig) error {
	if err := rf.PopulateAWS(aws); err != nil {
		return errors.Wrap(err, "error getting AWS secrets")
	}
	if err := rf.PopulateGithub(gh); err != nil {
		return errors.Wrap(err, "error getting GitHub secrets")
	}
	if err := rf.PopulateSlack(slack); err != nil {
		return errors.Wrap(err, "error getting Slack secrets")
	}
	if err := rf.PopulateServer(srv); err != nil {
		return errors.Wrap(err, "error getting server secrets")
	}
	if err := rf.PopulatePG(pg); err != nil {
		return errors.Wrap(err, "error getting db secrets")
	}
	return nil
}

// PopulatePG populates postgres secrets into pg
func (rf *ReadFileSecretsFetcher) PopulatePG(pg *config.PGConfig) error {
	s, err := ioutil.ReadFile(vaultSecretsRootPath + dbURIid)
	if err != nil {
		return errors.Wrap(err, "error getting DB URI")
	}
	pg.PostgresURI = string(s)
	return nil
}

// PopulateAWS populates AWS secrets into aws
func (rf *ReadFileSecretsFetcher) PopulateAWS(aws *config.AWSCreds) error {
	s, err := ioutil.ReadFile(vaultSecretsRootPath + awsAccessKeyIDid)
	if err != nil {
		return errors.Wrap(err, "error getting AWS access key ID")
	}
	aws.AccessKeyID = string(s)
	s, err = ioutil.ReadFile(vaultSecretsRootPath + awsSecretAccessKeyid)
	if err != nil {
		return errors.Wrap(err, "error getting AWS secret access key")
	}
	aws.SecretAccessKey = string(s)
	return nil
}

// PopulateGithub populates Github secrets into gh
func (rf *ReadFileSecretsFetcher) PopulateGithub(gh *config.GithubConfig) error {
	s, err := ioutil.ReadFile(vaultSecretsRootPath + githubHookSecretid)
	if err != nil {
		return errors.Wrap(err, "error getting GitHub hook secret")
	}
	gh.HookSecret = string(s)
	s, err = ioutil.ReadFile(vaultSecretsRootPath + githubTokenid)
	if err != nil {
		return errors.Wrap(err, "error getting GitHub token")
	}
	gh.Token = string(s)
	s, err = ioutil.ReadFile(vaultSecretsRootPath + githubAppID)
	if err != nil {
		return errors.Wrap(err, "error getting GitHub App ID")
	}
	// GitHub App
	appid, err := strconv.Atoi(string(s))
	if err != nil {
		return errors.Wrap(err, "app ID must be a valid integer")
	}
	if appid < 1 {
		return fmt.Errorf("app id must be >= 1: %v", appid)
	}
	gh.AppID = uint(appid)
	s, err = ioutil.ReadFile(vaultSecretsRootPath + githubAppPK)
	if err != nil {
		return errors.Wrap(err, "error getting GitHub App private key")
	}
	gh.PrivateKeyPEM = s
	s, err = ioutil.ReadFile(vaultSecretsRootPath + githubAppHookSecret)
	if err != nil {
		return errors.Wrap(err, "error getting GitHub App hook secret")
	}
	gh.AppHookSecret = string(s)
	// GitHub App OAuth
	s, err = ioutil.ReadFile(vaultSecretsRootPath + githubOAuthInstID)
	if err != nil {
		return errors.Wrap(err, "error getting GitHub App installation id")
	}
	iid, err := strconv.Atoi(string(s))
	if err != nil {
		return errors.Wrap(err, "error converting installation id into integer")
	}
	if iid < 1 {
		return fmt.Errorf("invalid installation id: %v", iid)
	}
	gh.OAuth.AppInstallationID = uint(iid)
	s, err = ioutil.ReadFile(vaultSecretsRootPath + githubOAuthClientID)
	if err != nil {
		return errors.Wrap(err, "error getting GitHub App client id")
	}
	gh.OAuth.ClientID = string(s)
	s, err = ioutil.ReadFile(vaultSecretsRootPath + githubOAuthClientSecret)
	if err != nil {
		return errors.Wrap(err, "error getting GitHub App client secret")
	}
	gh.OAuth.ClientSecret = string(s)
	s, err = ioutil.ReadFile(vaultSecretsRootPath + githubOAuthCookieAuthKey)
	if err != nil {
		return errors.Wrap(err, "error getting GitHub App cookie auth key")
	}
	if len(s) != 32 {
		return fmt.Errorf("bad cookie auth key: length must be exactly 32 bytes, value size: %v", len(s))
	}
	copy(gh.OAuth.CookieAuthKey[:], s)
	s, err = ioutil.ReadFile(vaultSecretsRootPath + githubOAuthCookieEncKey)
	if err != nil {
		return errors.Wrap(err, "error getting GitHub App cookie enc key")
	}
	if len(s) != 32 {
		return fmt.Errorf("bad cookie enc key: length must be exactly 32 bytes, value size: %v", len(s))
	}
	copy(gh.OAuth.CookieEncKey[:], s)
	s, err = ioutil.ReadFile(vaultSecretsRootPath + githubOAuthUserTokenEncKey)
	if err != nil {
		return errors.Wrap(err, "error getting GitHub App user token enc key")
	}
	if len(s) != 32 {
		return fmt.Errorf("bad user token enc key: length must be exactly 32 bytes, value size: %v", len(s))
	}
	copy(gh.OAuth.UserTokenEncKey[:], s)
	return nil
}

// PopulateSlack populates Slack secrets into slack
func (rf *ReadFileSecretsFetcher) PopulateSlack(slack *config.SlackConfig) error {
	s, err := ioutil.ReadFile(vaultSecretsRootPath + slackTokenid)
	if err != nil {
		return errors.Wrap(err, "error getting Slack token")
	}
	slack.Token = string(s)
	return nil
}

// PopulateServer populates server secrets into srv
func (rf *ReadFileSecretsFetcher) PopulateServer(srv *config.ServerConfig) error {
	s, err := ioutil.ReadFile(vaultSecretsRootPath + apiKeysid)
	if err != nil {
		return errors.Wrap(err, "error getting API keys")
	}
	srv.APIKeys = strings.Split(string(s), ",")
	if !srv.DisableTLS {
		s, err = ioutil.ReadFile(vaultSecretsRootPath + tlsCertid)
		if err != nil {
			return errors.Wrap(err, "error getting TLS certificate")
		}
		c := s
		s, err = ioutil.ReadFile(vaultSecretsRootPath + tlsKeyid)
		if err != nil {
			return errors.Wrap(err, "error getting TLS key")
		}
		k := s
		cert, err := tls.X509KeyPair(c, k)
		if err != nil {
			return errors.Wrap(err, "error parsing TLS cert/key")
		}
		srv.TLSCert = cert
	}
	return nil
}
