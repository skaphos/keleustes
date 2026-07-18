/*
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
*/

package cli

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/skaphos/keleustes/internal/api/openapi"
)

// defaultAPIBaseURL is where keleustesctl looks for the Keleustes API server
// when neither --api-url nor $KELEUSTES_API supplies one. It matches the
// server's default listen address (PROPOSAL §17).
const defaultAPIBaseURL = "http://localhost:8443/api/v1"

// apiBaseURL resolves the API server base URL with kubectl-style precedence:
// the --api-url flag first, then $KELEUSTES_API, then the localhost default.
// The resolved value is normalized so a bare host stays routable.
func apiBaseURL(cmd *cobra.Command) string {
	// The flag is registered persistently on root; on a subcommand it is
	// reachable through the merged flag set. A lookup miss falls through.
	raw := defaultAPIBaseURL
	if v, err := cmd.Flags().GetString("api-url"); err == nil && v != "" {
		raw = v
	} else if v := os.Getenv("KELEUSTES_API"); v != "" {
		raw = v
	}
	return normalizeBaseURL(raw)
}

// normalizeBaseURL makes a bare host URL usable. The server mounts the contract
// at /api/v1, so "http://host:8443" (no path) would otherwise request
// "/applications" and 404. When the URL carries no path, default it to /api/v1;
// an already path-qualified URL is left untouched.
func normalizeBaseURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw // leave malformed input for the client to surface
	}
	if strings.Trim(u.Path, "/") == "" {
		u.Path = "/api/v1"
	}
	return u.String()
}

// newAPIClient builds a typed openapi client pointed at apiBaseURL. The
// request editor injects a static dev bearer token, mirroring the UI dev
// stub; real token acquisition lands with the server's auth middleware.
func newAPIClient(cmd *cobra.Command) (*openapi.ClientWithResponses, error) {
	authEditor := func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer dev")
		return nil
	}
	return openapi.NewClientWithResponses(apiBaseURL(cmd),
		openapi.WithRequestEditorFn(authEditor))
}
