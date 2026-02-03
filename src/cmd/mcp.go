package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	globalConfig "github.com/AzielCF/az-wap/config"
	"github.com/AzielCF/az-wap/ui/mcp"
	"github.com/AzielCF/az-wap/ui/rest/helpers"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start WhatsApp MCP server using SSE",
	Long:  `Start a WhatsApp MCP (Model Context Protocol) server using Server-Sent Events (SSE) transport. This allows AI agents to interact with WhatsApp through a standardized protocol.`,
	Run:   mcpServer,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.Flags().StringVar(&globalConfig.McpPort, "port", "8080", "Port for the SSE MCP server")
	mcpCmd.Flags().StringVar(&globalConfig.McpHost, "host", "localhost", "Host for the SSE MCP server")
}

func mcpServer(_ *cobra.Command, _ []string) {
	// Set auto reconnect to whatsapp server after booting
	go helpers.SetAutoConnectAfterBooting(appUsecase)

	// Set auto reconnect checking with a valid client reference
	startAutoReconnectCheckerIfClientAvailable()

	// Create MCP server with capabilities
	mcpServer := server.NewMCPServer(
		"WhatsApp Web Multidevice MCP Server",
		globalConfig.AppVersion,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
	)

	// Add all WhatsApp tools
	sendHandler := mcp.InitMcpSend(sendUsecase)
	sendHandler.AddSendTools(mcpServer)

	queryHandler := mcp.InitMcpQuery(userUsecase, messageUsecase)
	queryHandler.AddQueryTools(mcpServer)

	appHandler := mcp.InitMcpApp(appUsecase)
	appHandler.AddAppTools(mcpServer)

	groupHandler := mcp.InitMcpGroup(groupUsecase)
	groupHandler.AddGroupTools(mcpServer)

	// Create SSE server
	sseServer := server.NewSSEServer(
		mcpServer,
		server.WithBaseURL(fmt.Sprintf("http://%s:%s", globalConfig.McpHost, globalConfig.McpPort)),
		server.WithKeepAlive(true),
	)

	// Start the SSE server
	addr := fmt.Sprintf("%s:%s", globalConfig.McpHost, globalConfig.McpPort)
	logrus.Printf("Starting WhatsApp MCP SSE server on %s", addr)
	logrus.Printf("SSE endpoint: http://%s:%s/sse", globalConfig.McpHost, globalConfig.McpPort)
	logrus.Printf("Message endpoint: http://%s:%s/message", globalConfig.McpHost, globalConfig.McpPort)

	// Graceful shutdown handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logrus.Info("[MCP] Reception of termination signal, shutting down gracefully...")
		StopApp()
		os.Exit(0)
	}()

	if err := sseServer.Start(addr); err != nil {
		logrus.Fatalf("Failed to start SSE server: %v", err)
	}
}
