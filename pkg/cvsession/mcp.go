package cvsession

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-go-golems/go-go-goja/pkg/repldb"
	"github.com/go-go-golems/go-go-goja/pkg/replmcp"
	"github.com/go-go-golems/go-go-goja/pkg/replsession"
	"github.com/go-go-golems/goja-cv/pkg/cv/docs"
	guide "github.com/go-go-golems/goja-cv/pkg/doc"
	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pkg/errors"
)

// ReferenceURI is the MCP resource with the generated op reference.
const ReferenceURI = "cv://reference"

// GuideURIPrefix prefixes the MCP resources of the guidebook pages
// (pkg/doc, cv://guide/<slug>), the person-facing docs of goja-cv help.
const GuideURIPrefix = "cv://guide/"

// DocsURIPrefix prefixes the MCP resources of the docs topics (cv://docs/<id>).
const DocsURIPrefix = "cv://docs/"

// MCPServer is goja-cv's MCP server: go-go-goja's replmcp server over a cv
// App, with the cv eval tool, an open tool, the cv://reference resource and
// the cv-agent prompt.
type MCPServer struct {
	Server         *mcp.Server
	App            *App
	DefaultSession string
	limits         replmcp.Limits
}

// EvalInput is the input of the cv eval tool.
type EvalInput struct {
	Code      string `json:"code" jsonschema:"JavaScript program body (the body of an async function: use await and return)."`
	SessionID string `json:"sessionId,omitempty" jsonschema:"Optional; omit it to use the default session (where img and mem live); only pass ids returned by create_session or list_sessions."`
}

// OpenInput is the input of the open tool. The descriptions say when to
// omit the optional fields: agents invent values for unexplained options.
type OpenInput struct {
	Path      string `json:"path" jsonschema:"Image file to open (PNG, JPEG, TIFF, LinearRaw DNG, .npy)."`
	Encoding  string `json:"encoding,omitempty" jsonschema:"Optional; omit it to use what the file says (photographs: srgb; float TIFF, DNG, .npy: linear). Pass linear only for data stored linearly in an integer file (scientific data, density maps, raw scans), srgb to treat linear-looking data as a photograph."`
	Name      string `json:"name,omitempty" jsonschema:"Optional; omit it to bind the image to img (replacing the current one). Pass a name (an identifier such as ref or scan) to keep img and open this image as a second global."`
	SessionID string `json:"sessionId,omitempty" jsonschema:"Optional; omit it to use the default session; only pass ids returned by create_session or list_sessions."`
}

// NewMCPServer creates the default session (with Options.Image bound to img)
// and the MCP server over app.
func NewMCPServer(ctx context.Context, app *App) (*MCPServer, error) {
	summary, err := app.CreateSession(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "create default cv session")
	}
	s := &MCPServer{
		App:            app,
		DefaultSession: summary.ID,
		limits:         replmcp.Limits{MaxResultChars: 20000, MaxImages: app.opts.MaxLooks, MaxImageBytes: app.opts.MaxLookBytes},
	}
	server, err := replmcp.NewServer(app,
		replmcp.WithName("goja-cv"),
		replmcp.WithVersion(cvmod.Version),
		replmcp.WithInstructions(Instructions),
		replmcp.WithDefaultSession(summary.ID),
		replmcp.WithLimits(s.limits),
		replmcp.WithExtraTools(s.register),
	)
	if err != nil {
		return nil, err
	}
	s.Server = server
	return s, nil
}

func (s *MCPServer) session(id string) string {
	if id = strings.TrimSpace(id); id != "" {
		return id
	}
	return s.DefaultSession
}

func (s *MCPServer) register(server *mcp.Server) {
	notDestructive := false
	// Replace replmcp's generic eval (whose description promises the last
	// expression as the result) with the cv eval tool.
	server.RemoveTools("eval")
	mcp.AddTool(server, &mcp.Tool{
		Name:        "eval",
		Description: EvalToolDescription,
		Annotations: &mcp.ToolAnnotations{Title: "Evaluate cv JavaScript", DestructiveHint: &notDestructive},
	}, s.eval)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "open",
		Description: "Open an image file and bind it to img (or to the name given as as) in the session; returns its summary and a first look.",
		Annotations: &mcp.ToolAnnotations{Title: "Open image", DestructiveHint: &notDestructive},
	}, s.open)
	server.AddResource(&mcp.Resource{
		URI:         ReferenceURI,
		Name:        "cv-reference",
		Title:       "cv op reference",
		Description: "Every cv op by receiver kind, with arguments and units, generated from the registry.",
		MIMEType:    "text/markdown",
	}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI: ReferenceURI, MIMEType: "text/markdown", Text: Reference(cvmod.DefaultRegistry()),
		}}}, nil
	})
	for _, t := range docs.Topics() {
		uri, body := DocsURIPrefix+t.ID, t.Body
		server.AddResource(&mcp.Resource{
			URI:         uri,
			Name:        "cv-docs-" + t.ID,
			Title:       t.Title,
			Description: t.Summary,
			MIMEType:    "text/markdown",
		}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "text/markdown", Text: body}}}, nil
		})
	}
	if pages, err := guide.Pages(); err == nil {
		for _, p := range pages {
			uri, body := GuideURIPrefix+p.Slug, "# "+p.Title+"\n\n"+p.Body
			server.AddResource(&mcp.Resource{
				URI:         uri,
				Name:        "cv-guide-" + p.Slug,
				Title:       p.Title,
				Description: p.Short + " (guidebook page; goja-cv help " + p.Slug + ")",
				MIMEType:    "text/markdown",
			}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "text/markdown", Text: body}}}, nil
			})
		}
	}
	server.AddPrompt(&mcp.Prompt{
		Name:        "cv-agent",
		Title:       "cv agent instructions",
		Description: "The eval rules and most used calls, followed by the full op reference.",
	}, func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: "cv agent instructions",
			Messages: []*mcp.PromptMessage{{
				Role:    "user",
				Content: &mcp.TextContent{Text: Instructions + "\n\n" + Reference(cvmod.DefaultRegistry())},
			}},
		}, nil
	})
}

func (s *MCPServer) eval(ctx context.Context, _ *mcp.CallToolRequest, in EvalInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Code) == "" {
		return nil, nil, errors.New("code is empty")
	}
	return s.run(ctx, s.session(in.SessionID), in.Code)
}

func (s *MCPServer) open(ctx context.Context, _ *mcp.CallToolRequest, in OpenInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Path) == "" {
		return nil, nil, errors.New("path is empty")
	}
	path, _ := json.Marshal(in.Path)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "img"
	}
	opts := map[string]any{"name": name}
	if e := strings.TrimSpace(in.Encoding); e != "" {
		opts["encoding"] = e
	}
	optJS, _ := json.Marshal(opts)
	code := "const opened = cv.open(" + string(path) + ", " + string(optJS) + ")\nreturn { image: await opened, look: opened.look(\"opened\") }"
	return s.run(ctx, s.session(in.SessionID), code)
}

// unknownSession is the tool error for an invented session id (the same
// wording as replmcp's, whose eval tool this server replaces).
func (s *MCPServer) unknownSession(sessionID string) error {
	ids := s.App.Sessions()
	var existing string
	switch {
	case len(ids) == 0:
		existing = "No sessions exist yet."
	case len(ids) == 1 && ids[0] == s.DefaultSession:
		existing = fmt.Sprintf("There is only the default session (%s).", ids[0])
	default:
		list := make([]string, len(ids))
		for i, id := range ids {
			list[i] = id
			if id == s.DefaultSession {
				list[i] += " (default)"
			}
		}
		existing = "Existing sessions: " + strings.Join(list, ", ") + "."
	}
	return errors.Errorf("session %q does not exist. %s To use the default session (where img, mem and your looks live), omit sessionId; to start a new one, call create_session. Only pass ids returned by create_session or list_sessions.", sessionID, existing)
}

func (s *MCPServer) run(ctx context.Context, sessionID, code string) (*mcp.CallToolResult, any, error) {
	resp, err := s.App.Evaluate(ctx, sessionID, code)
	if err != nil {
		if resp == nil || resp.Cell == nil {
			if errors.Is(err, replsession.ErrSessionNotFound) || errors.Is(err, repldb.ErrSessionNotFound) {
				return nil, nil, s.unknownSession(sessionID)
			}
			return nil, nil, errors.Wrapf(err, "evaluate in session %s", sessionID)
		}
		result := replmcp.EvalResult(resp, s.limits)
		result.IsError = true
		result.Content = append([]mcp.Content{&mcp.TextContent{Text: "session error: " + err.Error()}}, result.Content...)
		return result, nil, nil
	}
	return replmcp.EvalResult(resp, s.limits), nil, nil
}
