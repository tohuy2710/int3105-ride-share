package contracttest

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestBusinessFilesDoNotImportInfrastructure(t *testing.T) {
	paths, err := filepath.Glob("../*/service.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 5 {
		t.Fatal("service architecture check did not discover the expected services")
	}
	for _, path := range paths {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			name, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"database/sql", "github.com/gin-gonic/", "github.com/jackc/", "github.com/redis/", "github.com/rabbitmq/", "int3105-ride-share/internal/platform/api", "int3105-ride-share/internal/ride/postgres", "int3105-ride-share/internal/ride/redisgate"} {
				if strings.HasPrefix(name, forbidden) {
					t.Errorf("business file %s imports infrastructure %s", path, name)
				}
			}
		}
	}
}
