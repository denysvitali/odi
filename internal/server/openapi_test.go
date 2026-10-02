package server

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestOpenAPIProductionRouteParity(t *testing.T) {
	data, err := os.ReadFile("../../docs/openapi.yaml")
	require.NoError(t, err)
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	require.NoError(t, yaml.Unmarshal(data, &spec))
	server := &Server{e: gin.New(), apiToken: "synthetic-token"}
	server.initRoutes()
	parameter := regexp.MustCompile(`:([^/]+)`)
	actual := map[string]bool{}
	for _, route := range server.e.Routes() {
		path := parameter.ReplaceAllString(route.Path, "{$1}")
		method := strings.ToLower(route.Method)
		actual[method+" "+path] = true
		require.Contains(t, spec.Paths, path)
		require.Contains(t, spec.Paths[path], method, "undocumented operation %s %s", method, path)
	}
	for path, operations := range spec.Paths {
		for method := range operations {
			switch method {
			case "get", "post", "put", "patch", "delete", "head", "options":
				assert.True(t, actual[method+" "+path], "documented operation is unreachable: %s %s", method, path)
			}
		}
	}
}
