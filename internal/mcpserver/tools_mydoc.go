package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	mydocsvc "github.com/Tencent/WeKnora/internal/application/service/mydoc"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/mark3labs/mcp-go/mcp"
)

func mydocListTool() mcp.Tool {
	return mcp.NewTool(types.MCPEndpointToolMyDocList,
		mcp.WithDescription("List the caller's personal MyDoc tree (owned) or documents shared with them."),
		mcp.WithString("view", mcp.Description("owned (default) or shared")),
		readOnlyEndpointAnnotations(),
	)
}

func mydocMkdirTool() mcp.Tool {
	return mcp.NewTool(types.MCPEndpointToolMyDocMkdir,
		mcp.WithDescription("Create a folder in the caller's MyDoc hub."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Folder name")),
		mcp.WithString("parent_id", mcp.Description("Parent folder id; omit for root")),
	)
}

func mydocRenameTool() mcp.Tool {
	return mcp.NewTool(types.MCPEndpointToolMyDocRename,
		mcp.WithDescription("Rename a MyDoc node owned by the caller."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Node id")),
		mcp.WithString("name", mcp.Required(), mcp.Description("New name")),
	)
}

func mydocMoveTool() mcp.Tool {
	return mcp.NewTool(types.MCPEndpointToolMyDocMove,
		mcp.WithDescription("Move a MyDoc node under another folder."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Node id")),
		mcp.WithString("parent_id", mcp.Description("New parent folder id; empty for root")),
	)
}

func mydocArchiveTool() mcp.Tool {
	return mcp.NewTool(types.MCPEndpointToolMyDocArchive,
		mcp.WithDescription("Archive (hide) a MyDoc node and its descendants."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Node id")),
	)
}

func mydocReadTool() mcp.Tool {
	return mcp.NewTool(types.MCPEndpointToolMyDocRead,
		mcp.WithDescription("Read the text content of a MyDoc file the caller can access."),
		mcp.WithString("id", mcp.Required(), mcp.Description("File node id")),
		readOnlyEndpointAnnotations(),
	)
}

func mydocWriteTool() mcp.Tool {
	return mcp.NewTool(types.MCPEndpointToolMyDocWrite,
		mcp.WithDescription("Write a text/markdown artifact into the caller's MyDoc hub (kind=ai_artifact)."),
		mcp.WithString("filename", mcp.Required(), mcp.Description("File name, e.g. notes.md")),
		mcp.WithString("content", mcp.Required(), mcp.Description("File content")),
		mcp.WithString("source_agent", mcp.Required(), mcp.Description("Agent identifier writing the file")),
		mcp.WithString("parent_id", mcp.Description("Parent folder id")),
		mcp.WithString("mime", mcp.Description("MIME type; default text/markdown")),
	)
}

func (s *Server) mydocUser(ctx context.Context) (string, error) {
	if s.mydocService == nil {
		return "", &toolError{msg: "mydoc service unavailable"}
	}
	return mydocsvc.RequireHumanUser(ctx)
}

func (s *Server) handleMyDocList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	uid, err := s.mydocUser(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	view := strings.TrimSpace(req.GetString("view", types.MyDocViewOwned))
	nodes, err := s.mydocService.GetTree(ctx, uid, view)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	raw, _ := json.Marshal(nodes)
	return mcp.NewToolResultText(string(raw)), nil
}

func (s *Server) handleMyDocMkdir(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	uid, err := s.mydocUser(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}
	node, err := s.mydocService.Mkdir(ctx, uid, req.GetString("parent_id", ""), name)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	raw, _ := json.Marshal(node)
	return mcp.NewToolResultText(string(raw)), nil
}

func (s *Server) handleMyDocRename(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	uid, err := s.mydocUser(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}
	node, err := s.mydocService.Rename(ctx, uid, id, name)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	raw, _ := json.Marshal(node)
	return mcp.NewToolResultText(string(raw)), nil
}

func (s *Server) handleMyDocMove(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	uid, err := s.mydocUser(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}
	node, err := s.mydocService.Move(ctx, uid, id, req.GetString("parent_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	raw, _ := json.Marshal(node)
	return mcp.NewToolResultText(string(raw)), nil
}

func (s *Server) handleMyDocArchive(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	uid, err := s.mydocUser(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}
	if err := s.mydocService.Archive(ctx, uid, id); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(`{"success":true}`), nil
}

func (s *Server) handleMyDocRead(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	uid, err := s.mydocUser(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}
	rc, node, file, err := s.mydocService.ReadContent(ctx, uid, id, "")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, 2<<20))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	payload := map[string]any{
		"id": node.ID, "name": node.Name, "mime": file.Mime, "version": file.Version, "content": string(data),
	}
	raw, _ := json.Marshal(payload)
	return mcp.NewToolResultText(string(raw)), nil
}

func (s *Server) handleMyDocWrite(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	uid, err := s.mydocUser(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	filename, err := req.RequireString("filename")
	if err != nil {
		return mcp.NewToolResultError("filename is required"), nil
	}
	content, err := req.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError("content is required"), nil
	}
	sourceAgent, err := req.RequireString("source_agent")
	if err != nil || strings.TrimSpace(sourceAgent) == "" {
		return mcp.NewToolResultError("source_agent is required"), nil
	}
	mimeType := req.GetString("mime", "text/markdown")
	item, err := s.mydocService.UploadFile(
		ctx, uid, req.GetString("parent_id", ""), types.MyDocKindAIArtifact,
		filename, mimeType, []byte(content), sourceAgent, "",
	)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	raw, _ := json.Marshal(item)
	return mcp.NewToolResultText(string(raw)), nil
}
