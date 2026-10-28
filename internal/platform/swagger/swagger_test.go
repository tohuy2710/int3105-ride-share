package swagger

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"int3105-ride-share/docs/openapi"
	"int3105-ride-share/internal/platform/api"
)

func TestDocumentationContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, name := range []string{"auth", "ride", "location", "matching", "notification"} {
		t.Run(name, func(t *testing.T) {
			router := api.NewRouter(name+"-service", slog.New(slog.NewTextHandler(io.Discard, nil)))
			if err := Register(router, name+"-service"); err != nil {
				t.Fatal(err)
			}
			get := func(path string) *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				r := httptest.NewRequest("GET", path, nil)
				r.Header.Set("X-Request-ID", "swagger-test")
				router.ServeHTTP(w, r)
				if w.Header().Get("X-Request-ID") != "swagger-test" {
					t.Fatal("documentation lost request ID middleware")
				}
				return w
			}
			w := get("/swagger/openapi.json")
			if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("spec: %d %s", w.Code, w.Body.String())
			}
			doc, err := openapi3.NewLoader().LoadFromData(w.Body.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if err := doc.Validate(context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(doc.Servers) != 1 || doc.Servers[0].URL != "/" {
				t.Fatal("Try it out must target the serving origin")
			}
			source, err := openapi.Files.ReadFile(name + ".yaml")
			if err != nil {
				t.Fatal(err)
			}
			original, err := openapi3.NewLoader().LoadFromData(source)
			if err != nil {
				t.Fatal(err)
			}
			doc.Servers = original.Servers
			actual, _ := json.Marshal(doc)
			expected, _ := json.Marshal(original)
			if !bytes.Equal(actual, expected) {
				t.Fatal("served spec differs from published contract beyond servers")
			}
			if w := get("/swagger/openapi.yaml"); w.Code != 200 || !bytes.Equal(w.Body.Bytes(), source) {
				t.Fatal("raw YAML must match published contract exactly")
			}
			for _, path := range []string{"/swagger", "/swagger/"} {
				if w := get(path); w.Code != 307 || w.Header().Get("Location") != "/swagger/index.html" {
					t.Fatalf("missing documentation redirect: %s", path)
				}
			}
			for path, mime := range map[string]string{
				"index.html": "text/html", "swagger-ui.css": "text/css", "swagger-ui-bundle.js": "text/javascript",
			} {
				if w := get("/swagger/" + path); w.Code != 200 || w.Body.Len() == 0 || !strings.HasPrefix(w.Header().Get("Content-Type"), mime) {
					t.Fatalf("missing bundled asset: %s", path)
				}
			}
			for _, path := range []string{"/swagger/unknown", "/swagger/../LICENSE"} {
				if w := get(path); w.Code != 404 || !strings.Contains(w.Body.String(), `"code":"NOT_FOUND"`) {
					t.Fatalf("unexpected asset exposed: %s", path)
				}
			}
		})
	}
}

func TestUnknownServiceFailsRegistration(t *testing.T) {
	if err := Register(gin.New(), "unknown-service"); err == nil {
		t.Fatal("expected a missing-contract error")
	}
}
