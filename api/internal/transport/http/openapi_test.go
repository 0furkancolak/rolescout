package httptransport

import (
	"bufio"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

var (
	specPathLine   = regexp.MustCompile(`^  (/\S*):$`)
	specMethodLine = regexp.MustCompile(`^    (get|put|post|patch|delete):$`)
	routeParam     = regexp.MustCompile(`:(\w+)`)
)

// TestOpenAPIMatchesRoutes keeps docs/api/openapi.yaml in sync with the
// registered routes in both directions.
func TestOpenAPIMatchesRoutes(t *testing.T) {
	file, err := os.Open("../../../../docs/api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var documented []string
	var path string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if m := specPathLine.FindStringSubmatch(line); m != nil {
			path = m[1]
		} else if m := specMethodLine.FindStringSubmatch(line); m != nil && path != "" {
			documented = append(documented, strings.ToUpper(m[1])+" "+path)
		} else if line == "components:" {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	RegisterHealthRoutes(app, stubPinger{})
	RegisterAPIRoutes(app, Deps{Workspaces: allowAll})
	var registered []string
	for _, r := range app.GetRoutes(true) {
		if r.Method == fiber.MethodHead {
			continue // Fiber adds HEAD for every GET.
		}
		registered = append(registered, r.Method+" "+routeParam.ReplaceAllString(r.Path, "{$1}"))
	}

	slices.Sort(documented)
	slices.Sort(registered)
	for _, r := range registered {
		if !slices.Contains(documented, r) {
			t.Errorf("route %s is not documented in openapi.yaml", r)
		}
	}
	for _, d := range documented {
		if !slices.Contains(registered, d) {
			t.Errorf("openapi.yaml documents %s, which is not registered", d)
		}
	}
	if len(registered) == 0 {
		t.Fatal("no routes registered")
	}
}
