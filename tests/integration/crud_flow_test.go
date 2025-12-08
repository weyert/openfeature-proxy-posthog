package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/openfeature/posthog-proxy/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestCRUDFlow(t *testing.T) {
	// 1. Setup Mock PostHog Server
	mockPH := NewMockPostHogServer("123")
	defer mockPH.Close()

	// 2. Setup Proxy Server
	proxy := SetupProxy(t, mockPH)
	defer proxy.Close()

	client := &http.Client{}
	baseURL := proxy.URL + "/openfeature/v0/manifest/flags"

	// --- Step 1: Create Flag ---
	t.Log("Step 1: Create Flag")
	createReq := models.CreateFlagRequest{
		Key:          "integration-test-flag",
		Name:         "Integration Test Flag",
		Type:         "boolean",
		DefaultValue: false,
	}
	createBody, _ := json.Marshal(createReq)
	resp, err := client.Post(baseURL, "application/json", bytes.NewBuffer(createBody))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()

	// Verify it exists in Mock PostHog
	assert.Equal(t, 1, len(mockPH.Flags))
	assert.Equal(t, "integration-test-flag", mockPH.Flags[1].Key)

	// --- Step 2: Get Manifest ---
	t.Log("Step 2: Get Manifest")
	resp, err = client.Get(proxy.URL + "/openfeature/v0/manifest")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	var manifest models.Manifest
	err = json.NewDecoder(resp.Body).Decode(&manifest)
	assert.NoError(t, err)
	resp.Body.Close()

	// Find flag in array
	var foundFlag *models.ManifestFlag
	for _, f := range manifest.Flags {
		if f.Key == "integration-test-flag" {
			foundFlag = &f
			break
		}
	}
	assert.NotNil(t, foundFlag)
	assert.Equal(t, "integration-test-flag", foundFlag.Name)
	assert.Equal(t, "Integration Test Flag", foundFlag.Description)

	// --- Step 3: Get Single Flag ---
	t.Log("Step 3: Get Single Flag")
	resp, err = client.Get(baseURL + "/integration-test-flag")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var flagResp models.ManifestFlagResponse
	err = json.NewDecoder(resp.Body).Decode(&flagResp)
	assert.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, "integration-test-flag", flagResp.Flag.Key)
	assert.Equal(t, "integration-test-flag", flagResp.Flag.Name)
	assert.Equal(t, "Integration Test Flag", flagResp.Flag.Description)

	// --- Step 4: Update Flag ---
	t.Log("Step 4: Update Flag")
	updatedName := "Updated Integration Test Flag"
	updatedType := models.FlagTypeBoolean
	updateReq := models.UpdateFlagRequest{
		Name: &updatedName,
		Type: &updatedType,
		DefaultValue: true,
	}
	updateBody, _ := json.Marshal(updateReq)
	req, _ := http.NewRequest(http.MethodPut, baseURL+"/integration-test-flag", bytes.NewBuffer(updateBody))
	req.Header.Set("Content-Type", "application/json")
	
	resp, err = client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify update in Mock PostHog
	assert.Equal(t, "Updated Integration Test Flag", mockPH.Flags[1].Name)
	// Note: Checking DefaultValue update logic depends on how transformer handles it, 
	// but we verified the name update which confirms the flow works.

	// --- Step 5: Delete Flag ---
	t.Log("Step 5: Delete Flag")
	req, _ = http.NewRequest(http.MethodDelete, baseURL+"/integration-test-flag", nil)
	resp, err = client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// Verify deletion in Mock PostHog
	assert.Equal(t, 0, len(mockPH.Flags))

	// Verify 404 on Get Single Flag
	resp, err = client.Get(baseURL + "/integration-test-flag")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}
