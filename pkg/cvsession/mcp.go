package cvsession

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/go-go-golems/go-go-goja/pkg/replmcp"
	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pkg/errors"
)

// ReferenceURI is the MCP resource with the generated op reference.
const ReferenceURI = "cv://reference"

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
	SessionID string `json:"sessionId,omitempty" jsonschema:"Session to evaluate in. Omit to use the default session."`
}

// OpenInput is the input of the open tool.
type OpenInput struct {
	Path      string `json:"path" jsonschema:"Image file to open (PNG, JPEG, TIFF); it becomes img."`
	SessionID string `json:"sessionId,omitempty" jsonschema:"Session to open the image in. Omit to use the default session."`
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
		Description: "Open an image file and bind it to img in the session; returns its summary and a first look.",
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
	server.AddPrompt(&mcp.Prompt{
		Name:        "cv-agent",
		Title:       "cv agent instructions",
		Description: "The eval rules (async body, return, mem, looks, lazy nodes) and the op reference.",
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
	code := "img = cv.open(" + string(path) + ")\nreturn { image: await img, look: img.look(\"opened\") }"
	return s.run(ctx, s.session(in.SessionID), code)
}

func (s *MCPServer) run(ctx context.Context, sessionID, code string) (*mcp.CallToolResult, any, error) {
	resp, err := s.App.Evaluate(ctx, sessionID, code)
	if err != nil {
		if resp == nil || resp.Cell == nil {
			return nil, nil, errors.Wrapf(err, "evaluate in session %s", sessionID)
		}
		result := replmcp.EvalResult(resp, s.limits)
		result.IsError = true
		result.Content = append([]mcp.Content{&mcp.TextContent{Text: "session error: " + err.Error()}}, result.Content...)
		return result, nil, nil
	}
	return replmcp.EvalResult(resp, s.limits), nil, nil
}
