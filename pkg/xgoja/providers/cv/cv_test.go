package cv

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/go-go-goja/pkg/xgoja/app"
	"github.com/go-go-golems/go-go-goja/pkg/xgoja/providerapi"
	xmcp "github.com/go-go-golems/go-go-goja/pkg/xgoja/providers/mcp"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegister(t *testing.T) {
	registry := providerapi.NewProviderRegistry()
	if err := Register(registry); err != nil {
		t.Fatalf("register: %v", err)
	}
	mod, ok := registry.ResolveModule(PackageID, ModuleName)
	if !ok {
		t.Fatalf("module %s.%s not registered", PackageID, ModuleName)
	}
	if mod.DefaultAs != "cv" || mod.NewModuleFactory == nil || !json.Valid(mod.ConfigSchema) {
		t.Fatalf("module %+v", mod)
	}
	if _, err := mod.NewModuleFactory(providerapi.ModuleSetupContext{Config: json.RawMessage(`{"cacheBytez": 1}`)}); err == nil {
		t.Fatal("unknown config field accepted")
	}
	if err := Register(registry); err == nil {
		t.Fatal("duplicate registration accepted")
	}
}

// TestCVWithMCPCommandSet builds a runtime from an xgoja plan that selects the
// cv module and serves it through go-go-goja's mcp command set provider: a
// look returned from eval arrives as image content.
func TestCVWithMCPCommandSet(t *testing.T) {
	ctx := context.Background()
	providers := providerapi.NewProviderRegistry()
	if err := Register(providers); err != nil {
		t.Fatal(err)
	}
	if err := xmcp.Register(providers); err != nil {
		t.Fatal(err)
	}
	testdata, _ := filepath.Abs(filepath.Join("..", "..", "..", "..", "testdata"))
	plan := &app.RuntimePlan{Runtime: app.RuntimeSection{Modules: []app.RuntimeModulePlan{{Provider: PackageID, Name: ModuleName, As: "cv",
		Config: map[string]any{"root": t.TempDir(), "read": []any{testdata}}}}}}
	commandCtx := providerapi.CommandSetContext{
		Context: ctx, PackageID: xmcp.PackageID, Name: xmcp.CommandSetName, Mount: "mcp",
		Providers: providers, RuntimeFactory: app.NewRuntimeFactory(providers, plan, app.HostServices{}),
	}
	server, replApp, err := xmcp.NewServer(ctx, commandCtx, xmcp.Config{Name: "cv-agent"}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = replApp.Close(ctx) })
	st, ct := mcpsdk.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "t"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close(); _ = ss.Wait() })

	tiff, _ := filepath.Abs(filepath.Join("..", "..", "..", "..", "testdata", "gradient-64x48-16bit.tif"))
	code := `const cv = require("cv"); const img = cv.open(` + jsString(tiff) + `); ({ after: img.exposure(0.5).look("after") })`
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "eval", Arguments: map[string]any{"code": code}})
	if err != nil {
		t.Fatal(err)
	}
	text := res.Content[0].(*mcpsdk.TextContent).Text
	if res.IsError || !strings.Contains(text, "[content 1: look: after — open(gradient-64x48-16bit.tif) → exposure(0.5) (64×48, 1 image px = 1.0 doc px)]") || len(res.Content) != 2 {
		t.Fatalf("isError=%v content=%d text=%q", res.IsError, len(res.Content), text)
	}
	if img, ok := res.Content[1].(*mcpsdk.ImageContent); !ok || img.MIMEType != "image/png" {
		t.Fatalf("content[1] = %#v", res.Content[1])
	}
}

func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
