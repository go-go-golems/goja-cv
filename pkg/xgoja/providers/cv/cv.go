// Package cv is the xgoja provider for the goja-cv module: an xgoja.yaml that
// lists this provider can select module "cv" into a generated binary's
// runtime, so require("cv") is available to run, repl and the go-go-goja mcp
// command set.
package cv

import (
	"bytes"
	"encoding/json"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/go-go-golems/go-go-goja/pkg/xgoja/providerapi"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
	"github.com/pkg/errors"
)

// PackageID is the provider package id used in xgoja.yaml.
const PackageID = "goja-cv"

// ModuleName is the module the provider exposes.
const ModuleName = "cv"

// Config is the optional runtime config of the cv module.
type Config struct {
	// BaseDir resolves relative cv.open paths (default: the working directory).
	BaseDir string `json:"baseDir,omitempty"`
	// CacheBytes caps the value cache (default 2 GiB).
	CacheBytes int64 `json:"cacheBytes,omitempty"`
	// PreviewLong is the long side of the preview scale (default 1600).
	PreviewLong int `json:"previewLong,omitempty"`
	// LookLong is the long side of looks (default 1568).
	LookLong int `json:"lookLong,omitempty"`
}

// ConfigSchema is the JSON schema of Config.
var ConfigSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "baseDir": {"type": "string", "description": "Directory relative cv.open paths resolve against"},
    "cacheBytes": {"type": "integer", "minimum": 0, "description": "Byte cap of the value cache"},
    "previewLong": {"type": "integer", "minimum": 0, "description": "Long side of the preview scale in px"},
    "lookLong": {"type": "integer", "minimum": 0, "description": "Long side of looks in px"}
  }
}`)

// ParseConfig decodes raw module config; unknown fields are an error.
func ParseConfig(raw json.RawMessage) (Config, error) {
	var cfg Config
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return cfg, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, errors.Wrap(err, "goja-cv: cv module config")
	}
	return cfg, nil
}

// Register adds the goja-cv provider package with module "cv".
func Register(registry *providerapi.ProviderRegistry) error {
	return registry.Package(PackageID, providerapi.Module{
		Name:         ModuleName,
		DefaultAs:    ModuleName,
		Description:  cvmod.New().Doc(),
		ConfigSchema: ConfigSchema,
		NewModuleFactory: func(ctx providerapi.ModuleSetupContext) (require.ModuleLoader, error) {
			cfg, err := ParseConfig(ctx.Config)
			if err != nil {
				return nil, err
			}
			opts := graph.EngineOptions{BaseDir: cfg.BaseDir, CacheBytes: cfg.CacheBytes, PreviewLong: cfg.PreviewLong, LookLong: cfg.LookLong}
			mod := cvmod.New(cvmod.WithEngineFactory(func() *graph.Engine {
				return graph.NewEngine(cvmod.DefaultRegistry(), opts)
			}))
			// one engine per runtime: Loader builds it when the runtime first requires cv
			return func(vm *goja.Runtime, module *goja.Object) { mod.Loader(vm, module) }, nil
		},
	})
}
