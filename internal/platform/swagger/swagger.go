// Package swagger serves the published OpenAPI contracts and an offline UI.
package swagger

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"int3105-ride-share/docs/openapi"
	"int3105-ride-share/internal/platform/api"
)

//go:embed assets/*
var assets embed.FS

// Register exposes documentation for this service without changing API security.
func Register(router *gin.Engine, service string) error {
	name := strings.TrimSuffix(service, "-service")
	switch name {
	case "auth", "ride", "location", "matching", "notification":
	default:
		return fmt.Errorf("no OpenAPI contract for service %q", service)
	}
	source, err := openapi.Files.ReadFile(name + ".yaml")
	if err != nil {
		return err
	}
	doc, err := openapi3.NewLoader().LoadFromData(source)
	if err != nil {
		return fmt.Errorf("load %s OpenAPI: %w", name, err)
	}
	// Try it out must use the serving origin, including custom PORT/replicas.
	// The checked-in YAML remains unchanged as the published contract.
	doc.Servers = openapi3.Servers{&openapi3.Server{URL: "/"}}
	spec, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	type resource struct {
		body []byte
		mime string
	}
	resources := map[string]resource{
		"/openapi.json": {spec, "application/json; charset=utf-8"},
		"/openapi.yaml": {source, "application/yaml; charset=utf-8"},
	}
	for file, mime := range map[string]string{
		"index.html":           "text/html; charset=utf-8",
		"swagger-ui.css":       "text/css; charset=utf-8",
		"swagger-ui-bundle.js": "text/javascript; charset=utf-8",
		"LICENSE":              "text/plain; charset=utf-8",
		"NOTICE":               "text/plain; charset=utf-8",
	} {
		body, err := assets.ReadFile("assets/" + file)
		if err != nil {
			return err
		}
		resources["/"+file] = resource{body, mime}
	}
	router.GET("/swagger", func(c *gin.Context) { c.Redirect(http.StatusTemporaryRedirect, "/swagger/index.html") })
	router.GET("/swagger/*path", func(c *gin.Context) {
		if c.Param("path") == "/" {
			c.Redirect(http.StatusTemporaryRedirect, "/swagger/index.html")
			return
		}
		if r, ok := resources[c.Param("path")]; ok {
			c.Header("X-Content-Type-Options", "nosniff")
			c.Data(http.StatusOK, r.mime, r.body)
			return
		}
		api.Fail(c, http.StatusNotFound, "NOT_FOUND", "Documentation asset not found")
	})
	return nil
}
