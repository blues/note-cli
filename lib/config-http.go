// Copyright 2026 Blues Inc. All rights reserved.
// Use of this source code is governed by licenses granted by the copyright holder.

package lib

import (
	"context"
	"fmt"
	"net/http"

	"github.com/blues/note-go/notehub"
)

// ConfigDoAuthenticated is the common request path for Notehub operations.
// It renews expiring credentials before sending and retries a 401 once when a
// refresh succeeds and the request body can be replayed.
func ConfigDoAuthenticated(client *http.Client, req *http.Request) (*http.Response, error) {
	config, err := GetConfig()
	if err != nil {
		return nil, err
	}
	return config.doAuthenticated(client, req, config.Hub)
}

func (config *ConfigSettings) doAuthenticated(client *http.Client, req *http.Request, hub string) (*http.Response, error) {
	return notehub.DoAuthenticatedRequest(client, req, func(ctx context.Context, rejectedToken string) (string, error) {
		if _, err := config.RefreshCredentialsForHub(ctx, hub, rejectedToken); err != nil {
			return "", err
		}
		credentials := config.CredentialsForHub(hub)
		if credentials == nil {
			return "", fmt.Errorf("not logged in to %s: set %s, or run 'notehub login'", hub, ConfigTokenEnvVar)
		}
		return credentials.Token, nil
	})
}

// IntrospectCredentials checks the current credentials with Notehub using the
// same renewal and retry policy as data operations. Explicit PAT validation and
// OAuth exchanges keep using their supplied token, avoiding recursive refresh.
func (config *ConfigSettings) IntrospectCredentials(ctx context.Context, hub string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL(hub)+"/userinfo", nil)
	if err != nil {
		return "", err
	}
	response, err := config.doAuthenticated(http.DefaultClient, req, hub)
	if err != nil {
		return "", err
	}
	return introspectionResponse(response)
}
