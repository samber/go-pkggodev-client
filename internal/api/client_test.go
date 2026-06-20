package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/samber/go-pkggodev-client/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetPackage(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "godig-test", r.Header.Get("User-Agent"))
		assert.Equal(t, "/v1beta/package/github.com/samber/lo", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"path":"github.com/samber/lo","name":"lo"}`))
	}))
	defer srv.Close()

	c, err := api.New(
		api.WithBaseURL(srv.URL+"/v1beta"),
		api.WithUserAgent("godig-test"),
	)
	require.NoError(t, err)

	pkg, err := c.GetPackage(context.Background(), api.GetPackageParams{Path: "github.com/samber/lo"})
	require.NoError(t, err)
	assert.Equal(t, "github.com/samber/lo", pkg.Path.Value)
	assert.Equal(t, "lo", pkg.Name.Value)
}

func TestClient_DefaultBaseURL(t *testing.T) {
	t.Parallel()
	c, err := api.New()
	require.NoError(t, err)
	require.NotNil(t, c)
}
