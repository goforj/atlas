package mcp

import (
	"context"

	atlasdocs "github.com/goforj/atlas/docs"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// searchDocs runs bounded section-aware docs search.
func (s Server) searchDocs(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return toolError(err)
	}
	results, err := atlasdocs.Search(_context(ctx), s.docsProvider(), atlasdocs.SearchOptions{
		Query:      query,
		Limit:      request.GetInt("limit", 5),
		TokenLimit: request.GetInt("token_limit", 80),
	})
	if err != nil {
		return nil, err
	}
	return jsonResult(results)
}

// readDocSection returns one requested Markdown section instead of a whole document.
func (s Server) readDocSection(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil {
		return toolError(err)
	}
	section, ok, err := atlasdocs.ReadSection(_context(ctx), s.docsProvider(), path, request.GetString("heading", ""), request.GetInt("token_limit", 200))
	if err != nil {
		return nil, err
	}
	if !ok {
		return toolError(errNotFound("doc section"))
	}
	return jsonResult(section)
}

// readDocNeighborhood returns nearby sections when a target section needs local context.
func (s Server) readDocNeighborhood(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil {
		return toolError(err)
	}
	heading, err := request.RequireString("heading")
	if err != nil {
		return toolError(err)
	}
	sections, err := atlasdocs.ReadNeighborhood(
		_context(ctx),
		s.docsProvider(),
		path,
		heading,
		request.GetInt("before", 1),
		request.GetInt("after", 1),
		request.GetInt("token_limit", 120),
	)
	if err != nil {
		return nil, err
	}
	return jsonResult(sections)
}

// listDocHeadings gives agents a cheap outline before reading section bodies.
func (s Server) listDocHeadings(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil {
		return toolError(err)
	}
	headings, err := atlasdocs.ListHeadings(_context(ctx), s.docsProvider(), path)
	if err != nil {
		return nil, err
	}
	return jsonResult(headings)
}

// explainAPI maps GoForj commands and paths to likely docs sections.
func (s Server) explainAPI(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return toolError(err)
	}
	results, err := atlasdocs.ExplainAPI(_context(ctx), s.docsProvider(), query)
	if err != nil {
		return nil, err
	}
	return jsonResult(results)
}
