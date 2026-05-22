package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/atompi/changate/internal/model"
	_logger "github.com/atompi/changate/pkg/logger"
)

type MCPToolExecutor struct {
	httpClient *http.Client
	timeout    time.Duration
}

func NewMCPToolExecutor(timeout time.Duration) *MCPToolExecutor {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &MCPToolExecutor{
		httpClient: &http.Client{Timeout: timeout},
		timeout:    timeout,
	}
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type mcpResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (e *MCPToolExecutor) ExecuteTool(ctx context.Context, tool model.MCPTool, toolCall model.ToolCall) (string, error) {
	if tool.ServerURL == "" {
		return "", fmt.Errorf("no server URL configured for tool: %s", tool.ServerLabel)
	}

	args := json.RawMessage(toolCall.Arguments)
	params := toolCallParams{
		Name:      toolCall.Name,
		Arguments: args,
	}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tool params: %w", err)
	}

	reqBody := mcpRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  paramsBytes,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tool.ServerURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	if tool.Token != "" {
		req.Header.Set("Authorization", "Bearer "+tool.Token)
	}

	_logger.Debugf("[MCP] >>> %s %s", req.Method, tool.ServerURL)
	for k, v := range req.Header {
		_logger.Debugf("[MCP] >>> Header: %s=%s", k, v)
	}
	_logger.Debugf("[MCP] >>> Body: %s", string(bodyBytes))

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute tool: %w", err)
	}
	defer resp.Body.Close()

	_logger.Debugf("[MCP] <<< Status: %d", resp.StatusCode)
	for k, v := range resp.Header {
		_logger.Debugf("[MCP] <<< Header: %s=%s", k, v)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "text/event-stream" || resp.StatusCode == http.StatusOK {
		return e.parseSSEStream(resp)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	_logger.Debugf("[MCP] <<< Body: %s", string(respBody))

	var mcpResp mcpResponse
	if err := json.Unmarshal(respBody, &mcpResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tool execution failed with status: %d", resp.StatusCode)
	}

	if mcpResp.Error != nil {
		return "", fmt.Errorf("tool execution error: %s", mcpResp.Error.Message)
	}

	if len(mcpResp.Result.Content) == 0 {
		return "", fmt.Errorf("empty tool response")
	}

	return mcpResp.Result.Content[0].Text, nil
}

func (e *MCPToolExecutor) parseSSEStream(resp *http.Response) (string, error) {
	var fullData string
	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("failed to read SSE stream: %w", err)
		}

		_logger.Debugf("[MCP] <<< SSE Line: %s", string(line))

		if len(line) < 6 {
			continue
		}

		if line[:6] == "data: " {
			data := line[6:]
			data = trimSSEsuffix(data)
			fullData += data
		}
	}

	_logger.Debugf("[MCP] <<< SSE Data: %s", fullData)

	var mcpResp mcpResponse
	if err := json.Unmarshal([]byte(fullData), &mcpResp); err != nil {
		return "", fmt.Errorf("failed to decode SSE data: %w", err)
	}

	if mcpResp.Error != nil {
		return "", fmt.Errorf("tool execution error: %s", mcpResp.Error.Message)
	}

	if len(mcpResp.Result.Content) == 0 {
		return "", fmt.Errorf("empty tool response")
	}

	return mcpResp.Result.Content[0].Text, nil
}

func trimSSEsuffix(s string) string {
	s = strings.TrimRight(s, "\r\n")
	s = strings.TrimSuffix(s, "\n")
	s = strings.TrimSuffix(s, "\r")
	return s
}
