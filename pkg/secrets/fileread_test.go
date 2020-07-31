package secrets

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/dollarshaveclub/acyl/pkg/config"
)

func setupTestSecretsFile(t *testing.T, secretPath, value string) (*os.File, func(), error) {
	teardown := func() {}
	f, err := ioutil.TempFile(os.TempDir(), secretPath)
	if err != nil {
		f.WriteString(value)
		return f, teardown, err
	}
	teardown = func() {
		err := f.Close()
		if err != nil {
			t.Error("setup: Close:", err)
		}
	}
	return nil, teardown, nil
}

func TestReadFileSecretsFetcher(t *testing.T) {
	_, awsAccessKeyIDTeardown, err := setupTestSecretsFile(t, "aws/access_key_id", "fakeAwsKeyID")
	defer awsAccessKeyIDTeardown()
	if err != nil {
		t.Error("setup:", err)
	}
	_, awsSecretAccessKeyIDTeardown, err := setupTestSecretsFile(t, "aws/secret_access_key", "fakeAwsAccessKeyID")
	defer awsSecretAccessKeyIDTeardown()
	vaultConfig := &config.VaultConfig{
		SecretsRootPath: os.TempDir(),
		UseAgent:        true,
	}
	awsCreds := &config.AWSCreds{}
	sf := NewReadFileSecretsFetcher(vaultConfig)
	sf.PopulateAWS(awsCreds)

	if awsCreds.AccessKeyID != "fakeAwsKeyID" {
		t.Error("awsCreds.AccessKeyId not set correctly")
	}

	if awsCreds.SecretAccessKey != "fakeAwsAccessKeyID" {
		t.Error("awsCreds.fakeAwsAccessKeyID not set correctly")
	}

}
