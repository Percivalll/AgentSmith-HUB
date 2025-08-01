package mcp

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"AgentSmith-HUB/common"
	"AgentSmith-HUB/logger"
	"AgentSmith-HUB/project"
	"AgentSmith-HUB/rules_engine"

	"github.com/mark3labs/mcp-go/server"
)

// StandardMCPServer wraps the mcp-go server with our custom logic
type StandardMCPServer struct {
	server    *server.MCPServer
	apiMapper *APIMapper
	baseURL   string
	token     string
	ProjectMu sync.RWMutex
}

// NewStandardMCPServer creates a new server using mcp-go library
func NewStandardMCPServer() *StandardMCPServer {
	// Create mcp-go server
	s := server.NewMCPServer(
		"AgentSmith-HUB",
		"v0.1.7",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
	)

	std := &StandardMCPServer{
		server:    s,
		apiMapper: NewAPIMapper("http://localhost:8080", ""),
		baseURL:   "http://localhost:8080",
		token:     "",
	}

	logger.Info("Standard MCP server initialized with mcp-go")
	return std
}

// UpdateConfig updates server configuration
func (s *StandardMCPServer) UpdateConfig(baseURL, token string) {
	if baseURL != "" {
		s.baseURL = baseURL
		s.apiMapper.baseURL = baseURL
	}
	if token != "" {
		s.token = token
		s.apiMapper.token = token
	}
	logger.Info("Standard MCP server configuration updated", "baseURL", s.baseURL)
}

// GetMCPGoServer returns the underlying mcp-go server
func (s *StandardMCPServer) GetMCPGoServer() *server.MCPServer {
	return s.server
}

// GetAPIMapper returns the API mapper
func (s *StandardMCPServer) GetAPIMapper() *APIMapper {
	return s.apiMapper
}

// HandleJSONRPCRequest handles a JSON-RPC request using simplified logic
func (s *StandardMCPServer) HandleJSONRPCRequest(requestData []byte) ([]byte, error) {
	// Parse the JSON-RPC request
	var request map[string]interface{}
	if err := json.Unmarshal(requestData, &request); err != nil {
		return nil, fmt.Errorf("invalid JSON-RPC request: %v", err)
	}

	method, ok := request["method"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid method in request")
	}

	id := request["id"]

	// Handle different MCP methods
	switch method {
	case "initialize":
		return s.handleInitialize(id)
	case "tools/list":
		return s.handleToolsList(id)
	case "tools/call":
		return s.handleToolsCall(id, request)
	case "prompts/list":
		return s.handlePromptsList(id)
	case "prompts/get":
		return s.handlePromptsGet(id, request)
	case "resources/list":
		var p map[string]interface{}
		if m, ok := request["params"].(map[string]interface{}); ok {
			p = m
		} else {
			p = map[string]interface{}{}
		}
		return s.handleResourcesList(id, p)
	case "resources/read":
		return s.handleResourcesRead(id, request)
	default:
		// For other methods, return a generic success response
		return s.createJSONRPCResponse(id, map[string]interface{}{
			"status":  "success",
			"message": fmt.Sprintf("Method %s handled by mcp-go", method),
		})
	}
}

// handleInitialize handles the initialize method
func (s *StandardMCPServer) handleInitialize(id interface{}) ([]byte, error) {
	// Simple, concise initialization message
	instructions := "AgentSmith-HUB MCP Server ready. Use tools to interact with the system."

	// Return protocol information with minimal introduction
	result := map[string]interface{}{
		"protocolVersion": common.MCPVersion,
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{
				"listChanged": true,
			},
			"resources": map[string]interface{}{
				"listChanged": true,
				"subscribe":   true,
			},
			"prompts": map[string]interface{}{
				"listChanged": true,
			},
		},
		"serverInfo": map[string]interface{}{
			"name":         "AgentSmith-HUB",
			"version":      "v0.1.7",
			"instructions": instructions,
		},
	}
	return s.createJSONRPCResponse(id, result)
}

// handleToolsList handles the tools/list method
func (s *StandardMCPServer) handleToolsList(id interface{}) ([]byte, error) {
	tools := s.apiMapper.GetAllAPITools()
	var toolsList []map[string]interface{}

	for _, tool := range tools {
		// Convert tool schema to JSON Schema format
		properties := make(map[string]interface{})
		required := make([]string, 0)

		for argName, argDef := range tool.InputSchema {
			properties[argName] = map[string]interface{}{
				"type":        argDef.Type,
				"description": argDef.Description,
			}
			if argDef.Required {
				required = append(required, argName)
			}
		}

		schema := map[string]interface{}{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			schema["required"] = required
		}

		toolsList = append(toolsList, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": schema,
		})
	}

	result := map[string]interface{}{
		"tools": toolsList,
	}
	return s.createJSONRPCResponse(id, result)
}

// handleToolsCall handles the tools/call method
func (s *StandardMCPServer) handleToolsCall(id interface{}, request map[string]interface{}) ([]byte, error) {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return s.createJSONRPCError(id, -32602, "Invalid params", "Missing or invalid params")
	}

	toolName, ok := params["name"].(string)
	if !ok {
		return s.createJSONRPCError(id, -32602, "Invalid params", "Missing tool name")
	}

	arguments := make(map[string]interface{})
	if args, ok := params["arguments"].(map[string]interface{}); ok {
		arguments = args
	}

	// Call the API tool through our mapper
	result, err := s.apiMapper.CallAPITool(toolName, arguments)
	if err != nil {
		return s.createJSONRPCError(id, -32603, "Internal error", err.Error())
	}

	if result.IsError {
		errorMsg := "Tool execution failed"
		if len(result.Content) > 0 {
			errorMsg = result.Content[0].Text
		}
		return s.createJSONRPCError(id, -32603, "Tool execution failed", errorMsg)
	}

	// Convert tool result to MCP format
	var content []map[string]interface{}
	for _, c := range result.Content {
		content = append(content, map[string]interface{}{
			"type": "text", // MCP spec: content type should be "text" or "image"
			"text": c.Text,
		})
	}

	toolResult := map[string]interface{}{
		"content": content,
		"isError": false,
	}

	return s.createJSONRPCResponse(id, toolResult)
}

// handlePromptsList handles the prompts/list method
func (s *StandardMCPServer) handlePromptsList(id interface{}) ([]byte, error) {
	// Load prompts using unified config provider
	prompts, err := LoadMCPPrompts()
	if err != nil {
		return s.createJSONRPCError(id, -32603, "Failed to load prompts", err.Error())
	}

	var promptsList []map[string]interface{}
	for _, prompt := range prompts {
		// Convert arguments to JSON Schema format
		var arguments []map[string]interface{}
		for _, arg := range prompt.Arguments {
			argDef := map[string]interface{}{
				"name":        arg.Name,
				"description": arg.Description,
				"required":    arg.Required,
			}
			arguments = append(arguments, argDef)
		}

		promptsList = append(promptsList, map[string]interface{}{
			"name":        prompt.Name,
			"description": prompt.Description,
			"arguments":   arguments,
		})
	}

	result := map[string]interface{}{
		"prompts": promptsList,
	}
	return s.createJSONRPCResponse(id, result)
}

// handlePromptsGet handles the prompts/get method
func (s *StandardMCPServer) handlePromptsGet(id interface{}, request map[string]interface{}) ([]byte, error) {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return s.createJSONRPCError(id, -32602, "Invalid params", "Missing or invalid params")
	}

	promptName, ok := params["name"].(string)
	if !ok {
		return s.createJSONRPCError(id, -32602, "Invalid params", "Missing prompt name")
	}

	// optional args
	lang := ""
	if l, ok := params["lang"].(string); ok {
		lang = l
	}
	placeholders := map[string]interface{}{}
	if ph, ok := params["placeholders"].(map[string]interface{}); ok {
		placeholders = ph
	}

	prompt, err := GetMCPPrompt(promptName)
	if err != nil {
		return s.createJSONRPCError(id, -32602, "Prompt not found", fmt.Sprintf("Prompt '%s' not found: %v", promptName, err))
	}

	// choose text
	text := prompt.Template
	if lang != "" && prompt.Texts != nil {
		if t, ok := prompt.Texts[lang]; ok {
			text = t
		}
	}

	// placeholder replacement
	re := regexp.MustCompile(`\{\{(.*?)\}\}`)
	missing := []string{}
	rendered := re.ReplaceAllStringFunc(text, func(m string) string {
		key := strings.TrimSpace(re.ReplaceAllString(m, "$1"))
		if v, ok := placeholders[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		missing = append(missing, key)
		return m
	})
	if len(missing) > 0 {
		return s.createJSONRPCError(id, -32602, "Missing placeholders", strings.Join(missing, ","))
	}

	promptResult := map[string]interface{}{
		"description": prompt.Description,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": map[string]interface{}{
					"type": "text",
					"text": rendered,
				},
			},
		},
		"placeholders": prompt.Placeholders,
	}

	return s.createJSONRPCResponse(id, promptResult)
}

// handleResourcesList handles the resources/list method with enhanced accuracy and timeliness
func (s *StandardMCPServer) handleResourcesList(id interface{}, params map[string]interface{}) ([]byte, error) {
	var resources []map[string]interface{}

	// Parse optional params (type, limit, offset)
	filterType := ""
	limit := 100
	offset := 0

	if v, ok := params["type"].(string); ok {
		filterType = v
	}
	if v, ok := params["limit"].(float64); ok {
		limit = int(v)
	}
	if v, ok := params["offset"].(float64); ok {
		offset = int(v)
	}

	// Get real-time projects from API for accuracy
	if projectResult, err := s.apiMapper.CallAPITool("get_projects", map[string]interface{}{}); err == nil && !projectResult.IsError {
		if len(projectResult.Content) > 0 {
			var projects []map[string]interface{}
			if err := json.Unmarshal([]byte(projectResult.Content[0].Text), &projects); err == nil {
				for _, project := range projects {
					if projectId, ok := project["id"].(string); ok {
						// Get real-time project status for accuracy
						status := "unknown"
						if projectStatus, ok := project["status"].(string); ok {
							status = projectStatus
						}

						// Check for pending changes to indicate temporary state
						hasPendingChanges := false
						if pendingResult, err := s.apiMapper.CallAPITool("get_pending_changes", map[string]interface{}{}); err == nil {
							// Simple check if project has pending changes
							hasPendingChanges = strings.Contains(pendingResult.Content[0].Text, projectId)
						}

						resources = append(resources, map[string]interface{}{
							"uri":         fmt.Sprintf("hub://project/%s", projectId),
							"name":        fmt.Sprintf("Project: %s", projectId),
							"description": fmt.Sprintf("Project configuration and data flow for %s (Status: %s)", projectId, status),
							"mimeType":    "application/yaml",
							"annotations": map[string]interface{}{
								"type":              "project",
								"status":            status,
								"hasPendingChanges": hasPendingChanges,
								"lastUpdated":       time.Now().Format(time.RFC3339),
							},
						})
					}
				}
			}
		}
	}

	// Enhanced ruleset resources with real-time data
	project.ForEachRuleset(func(rsID string, rs *rules_engine.Ruleset) bool {
		// Dynamically find projects using this ruleset with error handling
		var owners []string
		func() {
			defer func() {
				if r := recover(); r != nil {
					owners = []string{} // fallback to empty list on error
				}
			}()
			owners = findProjectsUsingRuleset(rsID)
		}()

		// Get real-time sample data with enhanced metadata
		sampleCnt := 0
		lastSampleTime := ""
		sampler := common.GetSampler("ruleset." + rsID)
		if sampler != nil {
			stats := sampler.GetStats()
			sampleCnt = int(stats.SampledCount)
			// Note: LastSampleTime may not be available in current SamplerStats
			// We'll use current time as a placeholder for now
			if sampleCnt > 0 {
				lastSampleTime = time.Now().Format(time.RFC3339)
			}
		}

		// Check for pending changes to this ruleset
		hasPendingChanges := false
		if pendingResult, err := s.apiMapper.CallAPITool("get_pending_changes", map[string]interface{}{}); err == nil {
			hasPendingChanges = strings.Contains(pendingResult.Content[0].Text, rsID)
		}

		resources = append(resources, map[string]interface{}{
			"uri":         fmt.Sprintf("hub://ruleset/%s", rsID),
			"name":        fmt.Sprintf("Ruleset: %s", rsID),
			"description": fmt.Sprintf("Ruleset definition with %d rules (Samples: %d)", len(rs.Rules), sampleCnt),
			"mimeType":    "application/xml",
			"annotations": map[string]interface{}{
				"type":              "ruleset",
				"ownerProjects":     owners,
				"sampleCount":       sampleCnt,
				"lastSampleTime":    lastSampleTime,
				"ruleCount":         len(rs.Rules),
				"hasPendingChanges": hasPendingChanges,
				"lastUpdated":       time.Now().Format(time.RFC3339),
			},
		})
		return true
	})

	// Apply filtering & pagination
	filtered := make([]map[string]interface{}, 0)
	for _, r := range resources {
		if filterType != "" {
			if ann, ok := r["annotations"].(map[string]interface{}); ok {
				if t, ok := ann["type"].(string); ok && t != filterType {
					continue
				}
			}
		}
		filtered = append(filtered, r)
	}

	end := offset + limit
	if offset > len(filtered) {
		offset = len(filtered)
	}
	if end > len(filtered) {
		end = len(filtered)
	}
	paged := filtered[offset:end]

	result := map[string]interface{}{
		"resources": paged,
		"total":     len(filtered),
	}
	return s.createJSONRPCResponse(id, result)
}

// handleResourcesRead handles the resources/read method
func (s *StandardMCPServer) handleResourcesRead(id interface{}, request map[string]interface{}) ([]byte, error) {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return s.createJSONRPCError(id, -32602, "Invalid params", "Missing or invalid params")
	}

	uri, ok := params["uri"].(string)
	if !ok {
		return s.createJSONRPCError(id, -32602, "Invalid params", "Missing resource URI")
	}

	// Parse URI: hub://type/id
	if !strings.HasPrefix(uri, "hub://") {
		return s.createJSONRPCError(id, -32602, "Invalid URI", "URI must start with hub://")
	}

	var content string
	mimeType := "text/plain"

	// Split the URI into path and optional fragment (after #)
	var fragment string
	pathPart := uri
	if idx := strings.Index(uri, "#"); idx >= 0 {
		pathPart = uri[:idx]
		fragment = uri[idx+1:]
	}

	uriParts := strings.Split(strings.TrimPrefix(pathPart, "hub://"), "/")
	if len(uriParts) >= 2 {
		resType := uriParts[0]
		resID := uriParts[1]

		switch resType {
		case "project":
			// Get real-time project data for accuracy
			if projectResult, err := s.apiMapper.CallAPITool("get_project", map[string]interface{}{"id": resID}); err == nil && !projectResult.IsError {
				content = projectResult.Content[0].Text
				mimeType = "application/yaml"
			} else {
				return s.createJSONRPCError(id, -32602, "Project not found or inaccessible", uri)
			}
		case "ruleset":
			switch fragment {
			case "owners":
				// Dynamically find projects using this ruleset with error handling
				var owners []string
				func() {
					defer func() {
						if r := recover(); r != nil {
							owners = []string{} // fallback to empty list on error
						}
					}()
					owners = findProjectsUsingRuleset(resID)
				}()
				ownersJSON, _ := json.Marshal(owners)
				content = string(ownersJSON)
				mimeType = "application/json"
			case "samples":
				// Get sample data with MCP optimization (limited to 3 samples)
				sampler := common.GetSampler("ruleset." + resID)
				samplesJSON := "[]"
				if sampler != nil {
					allSamples := sampler.GetSamples()
					flat := make([]json.RawMessage, 0)
					count := 0
					maxSamples := 3 // MCP optimization: limit to 3 samples

					for _, list := range allSamples {
						for _, sm := range list {
							if count >= maxSamples {
								break
							}
							if b, err := json.Marshal(sm); err == nil {
								flat = append(flat, b)
								count++
							}
						}
					}
					samplesJSONBytes, _ := json.Marshal(flat)
					samplesJSON = string(samplesJSONBytes)
				}
				content = samplesJSON
				mimeType = "application/json"
			case "status":
				// New fragment: Get real-time ruleset status
				statusInfo := map[string]interface{}{
					"id":            resID,
					"lastUpdated":   time.Now().Format(time.RFC3339),
					"sampleCount":   0,
					"ownerProjects": findProjectsUsingRuleset(resID),
				}

				sampler := common.GetSampler("ruleset." + resID)
				if sampler != nil {
					stats := sampler.GetStats()
					statusInfo["sampleCount"] = int(stats.SampledCount)
				}

				// Check pending changes
				if pendingResult, err := s.apiMapper.CallAPITool("get_pending_changes", map[string]interface{}{}); err == nil {
					statusInfo["hasPendingChanges"] = strings.Contains(pendingResult.Content[0].Text, resID)
				}

				statusJSON, _ := json.Marshal(statusInfo)
				content = string(statusJSON)
				mimeType = "application/json"
			default:
				// Get real-time ruleset data for accuracy
				if rulesetResult, err := s.apiMapper.CallAPITool("get_ruleset", map[string]interface{}{"id": resID}); err == nil && !rulesetResult.IsError {
					// Extract XML content from the API response if it contains XML
					responseText := rulesetResult.Content[0].Text
					if strings.Contains(responseText, "<ruleset") {
						content = responseText
					} else {
						// Fallback to in-memory data if API response doesn't contain XML
						rs, ok := project.GetRuleset(resID)
						if !ok || rs == nil {
							return s.createJSONRPCError(id, -32602, "Ruleset not found", uri)
						}
						content = rs.RawConfig
					}
					mimeType = "application/xml"
				} else {
					return s.createJSONRPCError(id, -32602, "Ruleset not found or inaccessible", uri)
				}
			}
		}
	}

	if content == "" {
		content = "Resource content not available"
	}

	contents := []map[string]interface{}{
		{
			"uri":      uri,
			"mimeType": mimeType,
			"text":     content,
		},
	}

	result := map[string]interface{}{
		"contents": contents,
	}

	return s.createJSONRPCResponse(id, result)
}

// createJSONRPCResponse creates a JSON-RPC response
func (s *StandardMCPServer) createJSONRPCResponse(id interface{}, result interface{}) ([]byte, error) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	return json.Marshal(response)
}

// createJSONRPCError creates a JSON-RPC error response
func (s *StandardMCPServer) createJSONRPCError(id interface{}, code int, message string, data interface{}) ([]byte, error) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
			"data":    data,
		},
	}
	return json.Marshal(response)
}

// findProjectsUsingRuleset dynamically finds projects that use a specific ruleset
func findProjectsUsingRuleset(rulesetID string) []string {
	projects := make([]string, 0)

	project.ForEachProject(func(projectID string, proj *project.Project) bool {
		if _, exists := proj.Rulesets[rulesetID]; exists {
			projects = append(projects, projectID)
		}
		return true
	})

	return projects
}
