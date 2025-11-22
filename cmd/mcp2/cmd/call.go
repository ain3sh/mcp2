package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var (
	callPort     int
	callEndpoint string
	callTimeout  int
	jsonOutput   bool
)

var callCmd = &cobra.Command{
	Use:   "call",
	Short: "Call tools, prompts, or resources through the filtered mcp2 proxy",
	Long: `Call tools, prompts, or resources through mcp2's filtered view.
This uses the same filtering rules as the LLM-facing surface.

Available subcommands:
  tool     - Call a tool
  prompt   - Get a prompt
  resource - Read a resource`,
}

var callToolCmd = &cobra.Command{
	Use:   "tool --name <tool-name> --params <json>",
	Short: "Call a tool through the mcp2 proxy",
	Long: `Call a tool through the mcp2 proxy with the active profile's filtering rules.

Example:
  mcp2 call tool --name filesystem:list_directory --params '{"path":"/home/user"}'
  mcp2 call tool --name context7:get-library-docs --params '{"context7CompatibleLibraryID":"/websites/react_dev"}'`,
	RunE: runCallTool,
}

var callPromptCmd = &cobra.Command{
	Use:   "prompt --name <prompt-name> [--args <json>]",
	Short: "Get a prompt through the mcp2 proxy",
	Long: `Get a prompt through the mcp2 proxy with the active profile's filtering rules.

Example:
  mcp2 call prompt --name github:issue_template --args '{"repo":"ain3sh/mcp2"}'`,
	RunE: runCallPrompt,
}

var callResourceCmd = &cobra.Command{
	Use:   "resource --uri <resource-uri>",
	Short: "Read a resource through the mcp2 proxy",
	Long: `Read a resource through the mcp2 proxy with the active profile's filtering rules.

Example:
  mcp2 call resource --uri file:///home/user/projects/README.md`,
	RunE: runCallResource,
}

var (
	toolName     string
	toolParams   string
	promptName   string
	promptArgs   string
	resourceURI  string
)

func init() {
	rootCmd.AddCommand(callCmd)

	// Add subcommands
	callCmd.AddCommand(callToolCmd)
	callCmd.AddCommand(callPromptCmd)
	callCmd.AddCommand(callResourceCmd)

	// Common flags for all call subcommands
	for _, cmd := range []*cobra.Command{callToolCmd, callPromptCmd, callResourceCmd} {
		cmd.Flags().IntVar(&callPort, "port", 8210, "mcp2 server port")
		cmd.Flags().StringVar(&callEndpoint, "endpoint", "/mcp", "mcp2 endpoint (e.g., /mcp or /mcp/servername)")
		cmd.Flags().IntVar(&callTimeout, "timeout", 30, "request timeout in seconds")
		cmd.Flags().BoolVar(&jsonOutput, "json", false, "output raw JSON response")
	}

	// Tool-specific flags
	callToolCmd.Flags().StringVar(&toolName, "name", "", "tool name (required)")
	callToolCmd.Flags().StringVar(&toolParams, "params", "{}", "tool parameters as JSON")
	_ = callToolCmd.MarkFlagRequired("name")

	// Prompt-specific flags
	callPromptCmd.Flags().StringVar(&promptName, "name", "", "prompt name (required)")
	callPromptCmd.Flags().StringVar(&promptArgs, "args", "{}", "prompt arguments as JSON")
	_ = callPromptCmd.MarkFlagRequired("name")

	// Resource-specific flags
	callResourceCmd.Flags().StringVar(&resourceURI, "uri", "", "resource URI (required)")
	_ = callResourceCmd.MarkFlagRequired("uri")
}

// connectToMCP2 creates a client connection to the mcp2 server
func connectToMCP2(ctx context.Context) (*mcp.Client, *mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "mcp2-cli",
		Version: "0.1.0",
	}, nil)

	endpoint := fmt.Sprintf("http://127.0.0.1:%d%s", callPort, callEndpoint)
	transport := &mcp.StreamableClientTransport{
		Endpoint: endpoint,
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to mcp2 at %s: %w", endpoint, err)
	}

	return client, session, nil
}

func runCallTool(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(callTimeout)*time.Second)
	defer cancel()

	// Parse tool parameters
	var params map[string]any
	if err := json.Unmarshal([]byte(toolParams), &params); err != nil {
		return fmt.Errorf("invalid JSON in --params: %w", err)
	}

	// Connect to mcp2
	_, session, err := connectToMCP2(ctx)
	if err != nil {
		return err
	}
	defer session.Close()

	// Call the tool
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: params,
	})
	if err != nil {
		return fmt.Errorf("tool call failed: %w", err)
	}

	// Output results
	if jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Printf("Tool: %s\n", toolName)
		fmt.Printf("Status: Success\n")
		fmt.Printf("\nResult:\n")
		fmt.Printf("-------\n")

		if len(result.Content) == 0 {
			fmt.Println("(no content)")
		}

		for i, content := range result.Content {
			if textContent, ok := content.(*mcp.TextContent); ok {
				if len(result.Content) > 1 {
					fmt.Printf("\n[Content %d]\n", i)
				}
				fmt.Println(textContent.Text)
			} else if imageContent, ok := content.(*mcp.ImageContent); ok {
				fmt.Printf("\n[Image Content %d]\n", i)
				fmt.Printf("  Type: %s\n", imageContent.MIMEType)
				fmt.Printf("  Size: %d bytes\n", len(imageContent.Data))
			} else if embeddedResource, ok := content.(*mcp.EmbeddedResource); ok {
				fmt.Printf("\n[Embedded Resource %d]\n", i)
				if embeddedResource.Resource != nil && embeddedResource.Resource.URI != "" {
					fmt.Printf("  URI: %s\n", embeddedResource.Resource.URI)
				}
			}
		}

		if result.IsError {
			fmt.Println("\nNote: Tool indicated an error condition")
		}
	}

	return nil
}

func runCallPrompt(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(callTimeout)*time.Second)
	defer cancel()

	// Parse prompt arguments
	var promptArgsMap map[string]string
	if err := json.Unmarshal([]byte(promptArgs), &promptArgsMap); err != nil {
		return fmt.Errorf("invalid JSON in --args: %w", err)
	}

	// Connect to mcp2
	_, session, err := connectToMCP2(ctx)
	if err != nil {
		return err
	}
	defer session.Close()

	// Get the prompt
	result, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      promptName,
		Arguments: promptArgsMap,
	})
	if err != nil {
		return fmt.Errorf("prompt get failed: %w", err)
	}

	// Output results
	if jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Printf("Prompt: %s\n", promptName)
		fmt.Printf("Status: Success\n")

		if result.Description != "" {
			fmt.Printf("Description: %s\n", result.Description)
		}

		fmt.Printf("\nMessages:\n")
		fmt.Printf("---------\n")

		if len(result.Messages) == 0 {
			fmt.Println("(no messages)")
		}

		for i, msg := range result.Messages {
			fmt.Printf("\n[Message %d - Role: %s]\n", i, msg.Role)
			if textContent, ok := msg.Content.(*mcp.TextContent); ok {
				fmt.Println(textContent.Text)
			} else if imageContent, ok := msg.Content.(*mcp.ImageContent); ok {
				fmt.Printf("  [Image: %s, %d bytes]\n", imageContent.MIMEType, len(imageContent.Data))
			} else if embeddedResource, ok := msg.Content.(*mcp.EmbeddedResource); ok {
				fmt.Printf("  [Embedded Resource]\n")
				if embeddedResource.Resource != nil && embeddedResource.Resource.URI != "" {
					fmt.Printf("    URI: %s\n", embeddedResource.Resource.URI)
				}
			}
		}
	}

	return nil
}

func runCallResource(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(callTimeout)*time.Second)
	defer cancel()

	// Connect to mcp2
	_, session, err := connectToMCP2(ctx)
	if err != nil {
		return err
	}
	defer session.Close()

	// Read the resource
	result, err := session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: resourceURI,
	})
	if err != nil {
		return fmt.Errorf("resource read failed: %w", err)
	}

	// Output results
	if jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Printf("Resource: %s\n", resourceURI)
		fmt.Printf("Status: Success\n")
		fmt.Printf("\nContents:\n")
		fmt.Printf("---------\n")

		if len(result.Contents) == 0 {
			fmt.Println("(no contents)")
		}

		for i, content := range result.Contents {
			if len(result.Contents) > 1 {
				fmt.Printf("\n[Content %d - URI: %s]\n", i, content.URI)
			}

			// Check if it's text or blob
			if content.Text != "" {
				fmt.Println(content.Text)
				if content.MIMEType != "" {
					fmt.Printf("\nMIME Type: %s\n", content.MIMEType)
				}
			} else if len(content.Blob) > 0 {
				fmt.Printf("\n[Blob Content - URI: %s]\n", content.URI)
				if content.MIMEType != "" {
					fmt.Printf("  MIME Type: %s\n", content.MIMEType)
				}
				fmt.Printf("  Size: %d bytes\n", len(content.Blob))
				if !jsonOutput {
					fmt.Printf("  (binary data not displayed in text mode)\n")
				}
			}
		}
	}

	return nil
}

// Helper to print JSON output to stderr and exit with error
func printErrorJSON(message string, err error) {
	errObj := map[string]string{
		"error":   message,
		"details": err.Error(),
	}
	data, _ := json.MarshalIndent(errObj, "", "  ")
	fmt.Fprintln(os.Stderr, string(data))
}
