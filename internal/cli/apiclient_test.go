/*
SPDX-FileCopyrightText: 2026 Skaphos
SPDX-License-Identifier: MIT
*/

package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://host:8443", "http://host:8443/api/v1"},                          // bare host -> /api/v1
		{"http://host:8443/", "http://host:8443/api/v1"},                         // trailing slash counts as no path
		{"http://host:8443/api/v1", "http://host:8443/api/v1"},                   // already qualified -> untouched
		{"https://gw.example.com/keleustes", "https://gw.example.com/keleustes"}, // custom path -> untouched
		{"://nonsense", "://nonsense"},                                           // unparseable -> returned as-is
	}
	for _, c := range cases {
		if got := normalizeBaseURL(c.in); got != c.want {
			t.Errorf("normalizeBaseURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAPIBaseURLPrecedence(t *testing.T) {
	const flagURL = "http://flag.example/api/v1"
	const envURL = "http://env.example/api/v1"

	withFlag := func(v string) *cobra.Command {
		cmd := &cobra.Command{}
		cmd.Flags().String("api-url", v, "")
		return cmd
	}

	t.Run("flag wins over env", func(t *testing.T) {
		t.Setenv("KELEUSTES_API", envURL)
		if got := apiBaseURL(withFlag(flagURL)); got != flagURL {
			t.Errorf("got %q, want flag %q", got, flagURL)
		}
	})
	t.Run("env when flag empty", func(t *testing.T) {
		t.Setenv("KELEUSTES_API", envURL)
		if got := apiBaseURL(withFlag("")); got != envURL {
			t.Errorf("got %q, want env %q", got, envURL)
		}
	})
	t.Run("default when neither set", func(t *testing.T) {
		t.Setenv("KELEUSTES_API", "")
		if got := apiBaseURL(withFlag("")); got != defaultAPIBaseURL {
			t.Errorf("got %q, want default %q", got, defaultAPIBaseURL)
		}
	})
}
