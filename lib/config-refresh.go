// Copyright 2026 Blues Inc. All rights reserved.
// Use of this source code is governed by licenses granted by the copyright holder.

package lib

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/blues/note-go/notehub"
)

const credentialRefreshMargin = time.Minute

// ConfigRefreshCredentials silently renews the selected hub's saved OAuth login.
// A rejectedToken forces renewal after a 401, unless another CLI already replaced
// that token. The result says whether the caller should retry with new credentials.
func ConfigRefreshCredentials(ctx context.Context, rejectedToken string) (bool, error) {
	config, err := GetConfig()
	if err != nil {
		return false, err
	}
	return config.RefreshCredentials(ctx, rejectedToken)
}

func (config *ConfigSettings) RefreshCredentials(ctx context.Context, rejectedToken string) (bool, error) {
	return config.RefreshCredentialsForHub(ctx, config.Hub, rejectedToken)
}

// RefreshCredentialsForHub also supports status --all without changing the active
// connection or applying its environment override to other saved connections.
func (config *ConfigSettings) RefreshCredentialsForHub(ctx context.Context, hub, rejectedToken string) (bool, error) {
	credentials := config.CredentialsForHub(hub)
	if credentials == nil || credentials.Source != "" || !credentials.IsOAuthAccessToken() || credentials.RefreshToken == "" {
		return false, nil
	}
	needsRefresh := func(creds *ConfigCreds) bool {
		return creds.ExpiresAt == nil || !creds.ExpiresAt.After(time.Now().Add(credentialRefreshMargin))
	}
	if rejectedToken == "" && !needsRefresh(credentials) {
		return false, nil
	}
	changed := false
	err := config.withStoredCredentials(ctx, func(stored *ConfigSettings) (bool, error) {
		current := stored.SavedCredentials(hub)
		if current == nil {
			return false, fmt.Errorf("not logged in to %s; run 'notehub login'", hub)
		}
		changed = current.Token != credentials.Token || (rejectedToken != "" && current.Token != rejectedToken)
		if !current.IsOAuthAccessToken() || current.RefreshToken == "" || (!needsRefresh(current) && (rejectedToken == "" || current.Token != rejectedToken)) {
			return false, nil
		}
		token, err := notehub.RefreshAccessToken(ctx, hub, current.RefreshToken)
		if errors.Is(err, notehub.ErrRefreshTokenRejected) {
			return false, fmt.Errorf("%s: %w; run 'notehub login' to authorize a new session", hub, err)
		}
		if err != nil {
			// Early renewal is opportunistic while this access token is still
			// usable. Never hide rejection, cancellation, or a failure after expiry.
			if ctx.Err() == nil && rejectedToken != current.Token && current.ExpiresAt != nil && current.ExpiresAt.After(time.Now()) {
				return false, nil
			}
			return false, fmt.Errorf("refreshing login for %s: %w", hub, err)
		}
		current.Token, current.RefreshToken, current.ExpiresAt = token.AccessToken, token.RefreshToken, &token.ExpiresAt
		stored.HubCreds[hub] = *current
		changed = true
		return true, nil
	})
	return changed, err
}
