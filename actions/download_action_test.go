package actions_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-debos/debos"
	"github.com/go-debos/debos/actions"
	"github.com/stretchr/testify/assert"
)

func TestDownloadActionSha256sum(t *testing.T) {
	// Test HTTP server to serve files with this content
	testFileContent := []byte("This is a test file for sha256sum verification.")
	hasher := sha256.New()
	hasher.Write(testFileContent)
	expectedSha256sum := hex.EncodeToString(hasher.Sum(nil))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(testFileContent)
	}))
	defer ts.Close()

	// Temporary scratch directory
	tmpdir, err := os.MkdirTemp("", "debos-test-")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	context := &debos.DebosContext{
		CommonContext: &debos.CommonContext{
			Origins:    make(map[string]string),
			Scratchdir: tmpdir,
		},
		Architecture: "amd64",
	}

	// Test case 1: Correct sha256sum
	action1 := actions.DownloadAction{
		Url:       ts.URL + "/test-action1",
		Name:      "test-file-correct",
		Sha256sum: expectedSha256sum,
	}

	err = action1.Verify(context)
	assert.NoError(t, err, "Verify should pass for correct sha256sum")

	err = action1.Run(context)
	assert.NoError(t, err, "Run should pass for correct sha256sum")

	downloadedPath1, ok := context.Origins[action1.Name]
	assert.True(t, ok, "Origin path should be set")
	_, err = os.Stat(downloadedPath1)
	assert.NoError(t, err, "Downloaded file should exist")

	// Test case 2: Incorrect sha256sum
	action2 := actions.DownloadAction{
		Url:       ts.URL + "/test-action2",
		Name:      "test-file-incorrect",
		Sha256sum: "a" + expectedSha256sum[1:], // Mismatched SHA256 sum
	}

	err = action2.Verify(context)
	assert.NoError(t, err, "Verify should pass even with incorrect sum (runtime check)")

	err = action2.Run(context)
	assert.Error(t, err, "Run should fail for incorrect sha256sum")
	assert.Contains(t, err.Error(), "SHA256 sum mismatch")

	_, missing := context.Origins[action2.Name]
	assert.False(t, missing, "Origin path should not be set on failure")
	downloadedPath2 := tmpdir + "/" + action2.Name
	_, err = os.Stat(downloadedPath2)
	assert.True(t, os.IsNotExist(err), "Downloaded file should be removed on SHA256 sum mismatch")

	// Test case 3: Invalid sha256sum length in Verify
	action3 := actions.DownloadAction{
		Url:       ts.URL + "/test-action3",
		Name:      "test-file-invalid-len",
		Sha256sum: "abc", // Invalid length
	}
	err = action3.Verify(context)
	assert.Error(t, err, "Verify should fail for invalid sha256sum length")
	assert.Contains(t, err.Error(), "invalid length for property 'sha256sum'")

	// Test case 4: Invalid hex characters in Verify
	action4 := actions.DownloadAction{
		Url:       ts.URL + "/test-action4",
		Name:      "test-file-invalid-hex",
		Sha256sum: expectedSha256sum[:63] + "Z", // Invalid hex character
	}
	err = action4.Verify(context)
	assert.Error(t, err, "Verify should fail for invalid hex characters")
	assert.Contains(t, err.Error(), "invalid characters in 'sha256sum' property")

	// Test case 5: No sha256sum provided
	action5 := actions.DownloadAction{
		Url:  ts.URL + "/test-action5",
		Name: "test-file-no-sum",
	}

	err = action5.Verify(context)
	assert.NoError(t, err, "Verify should pass when no sha256sum is provided")

	err = action5.Run(context)
	assert.NoError(t, err, "Run should pass when no sha256sum is provided")

	downloadedPath5, ok := context.Origins[action5.Name]
	assert.True(t, ok, "Origin path should be set")
	_, err = os.Stat(downloadedPath5)
	assert.NoError(t, err, "Downloaded file should exist")
}
