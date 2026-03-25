// Package main provides an example agent integration using the WeChat SDK.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	weixinsdk "github.com/1ncludeSteven/weixin-sdk-go"
	"github.com/1ncludeSteven/weixin-sdk-go/pkg/api"
)

// MyAgent is a simple example agent that echoes messages.
type MyAgent struct {
	sdk       *weixinsdk.SDK
	accountID string
}

// NewMyAgent creates a new agent instance.
func NewMyAgent(accountID string) *MyAgent {
	return &MyAgent{
		sdk:       weixinsdk.New(nil),
		accountID: accountID,
	}
}

// HandleMessage handles incoming messages.
func (a *MyAgent) HandleMessage(ctx context.Context, msg *api.WeixinMessage, accountID string) error {
	// Extract text from message
	var textContent string
	for _, item := range msg.ItemList {
		if item.Type == int(api.MessageItemTypeText) && item.TextItem != nil {
			textContent = item.TextItem.Text
			break
		}
		if item.Type == int(api.MessageItemTypeVoice) && item.VoiceItem != nil && item.VoiceItem.Text != "" {
			textContent = item.VoiceItem.Text
			break
		}
	}

	if textContent == "" {
		return nil // No text content to respond to
	}

	fmt.Printf("[收到消息] from=%s text=%s\n", msg.FromUserID, textContent)

	// Store context token for reply
	if msg.ContextToken != "" {
		a.sdk.SetContextToken(accountID, msg.FromUserID, msg.ContextToken)
	}

	// Generate reply (simple echo for demo)
	reply := fmt.Sprintf("收到: %s", textContent)

	// Send reply
	contextToken := a.sdk.GetContextToken(accountID, msg.FromUserID)
	_, err := a.sdk.SendText(ctx, accountID, msg.FromUserID, reply, contextToken)
	if err != nil {
		fmt.Printf("[发送失败] %v\n", err)
		return err
	}

	fmt.Printf("[发送成功] to=%s\n", msg.FromUserID)
	return nil
}

// Start starts the agent.
func (a *MyAgent) Start(ctx context.Context) error {
	handler := func(ctx context.Context, msg *api.WeixinMessage, accountID string) error {
		return a.HandleMessage(ctx, msg, accountID)
	}

	monitor, err := a.sdk.NewMonitor(a.accountID, weixinsdk.WithHandler(handler))
	if err != nil {
		return fmt.Errorf("failed to create monitor: %w", err)
	}

	fmt.Printf("Agent started for account: %s\n", a.accountID)
	return monitor.Start(ctx)
}

func main() {
	// Get account ID from environment or argument
	accountID := os.Getenv("WEIXIN_ACCOUNT_ID")
	if accountID == "" && len(os.Args) > 1 {
		accountID = os.Args[1]
	}

	if accountID == "" {
		fmt.Fprintln(os.Stderr, "Usage: agent <account-id>")
		fmt.Fprintln(os.Stderr, "   or: set WEIXIN_ACCOUNT_ID environment variable")
		os.Exit(1)
	}

	// Create agent
	agent := NewMyAgent(accountID)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
	}()

	// Start agent
	fmt.Println("WeChat Agent Example")
	fmt.Println("====================")
	fmt.Println("This is a simple echo bot that responds to messages.")
	fmt.Println()

	if err := agent.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Agent error: %v\n", err)
		os.Exit(1)
	}
}
