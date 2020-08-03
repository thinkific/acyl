package secrets

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/dollarshaveclub/acyl/pkg/config"
)

const secretsRootPath = "./testdata/"

func setupTestSecretsFile(t *testing.T, secretPath, secretFile, value string) (*os.File, func(), error) {
	teardown := func() {}
	err := os.MkdirAll(secretPath, 0755)
	if err != nil {
		t.Fatal("Error creating secrets file directory: ", err)
	}
	err = ioutil.WriteFile(fmt.Sprintf("%v/%v", secretPath, secretFile), []byte(value), 0644)
	if err != nil {
		t.Error("setup: WriteFile:", err)
	}
	teardown = func() {
		err := os.Remove(fmt.Sprintf("%v/%v", secretPath, secretFile))
		if err != nil {
			t.Error("setup: Close:", err)
		}
	}
	return nil, teardown, nil
}

func TestReadFileSecretsFetcherPopulateAWS(t *testing.T) {
	_, awsAccessKeyIDTeardown, err := setupTestSecretsFile(t, "./testdata/aws", "access_key_id", "fakeAwsKeyID")
	defer awsAccessKeyIDTeardown()
	if err != nil {
		t.Error("setup:", err)
	}
	_, awsSecretAccessKeyIDTeardown, err := setupTestSecretsFile(t, "./testdata/aws", "secret_access_key", "fakeAwsAccessKeyID")
	defer awsSecretAccessKeyIDTeardown()
	vaultConfig := &config.VaultConfig{
		SecretsRootPath: secretsRootPath,
		UseAgent:        true,
	}
	awsCreds := &config.AWSCreds{}
	sf := NewReadFileSecretsFetcher(vaultConfig)
	sf.PopulateAWS(awsCreds)

	if awsCreds.AccessKeyID != "fakeAwsKeyID" {
		t.Errorf("awsCreds.AccessKeyId not set correctly -  expected %v, got %v", "fakeAwsKeyID", awsCreds.AccessKeyID)
	}

	if awsCreds.SecretAccessKey != "fakeAwsAccessKeyID" {
		t.Error("awsCreds.fakeAwsAccessKeyID not set correctly")
	}

}
