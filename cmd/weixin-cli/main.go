// Package main provides a CLI tool for WeChat SDK operations.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	weixinsdk "github.com/openclaw/weixin-sdk-go"
	"github.com/openclaw/weixin-sdk-go/pkg/api"
)

func main() {
	// Subcommands
	loginCmd := flag.NewFlagSet("login", flag.ExitOnError)
	loginForce := loginCmd.Bool("force", false, "Force new login even if one is in progress")
	loginAccount := loginCmd.String("account", "", "Account ID (optional, auto-generated if not provided)")

	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	sendAccount := sendCmd.String("account", "", "Account ID (required)")
	sendTo := sendCmd.String("to", "", "Target user ID (required)")
	sendText := sendCmd.String("text", "", "Text message to send")
	sendImage := sendCmd.String("image", "", "Image file path to send")

	monitorCmd := flag.NewFlagSet("monitor", flag.ExitOnError)
	monitorAccount := monitorCmd.String("account", "", "Account ID (required)")

	listCmd := flag.NewFlagSet("list", flag.ExitOnError)

	deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)
	deleteAccount := deleteCmd.String("account", "", "Account ID to delete (required)")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	sdk := weixinsdk.New(nil)

	switch os.Args[1] {
	case "login":
		loginCmd.Parse(os.Args[2:])
		doLogin(sdk, *loginForce, *loginAccount)

	case "send":
		sendCmd.Parse(os.Args[2:])
		doSend(sdk, *sendAccount, *sendTo, *sendText, *sendImage)

	case "monitor":
		monitorCmd.Parse(os.Args[2:])
		doMonitor(sdk, *monitorAccount)

	case "list":
		listCmd.Parse(os.Args[2:])
		doList(sdk)

	case "delete":
		deleteCmd.Parse(os.Args[2:])
		doDelete(sdk, *deleteAccount)

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("WeChat SDK CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  weixin-sdk login [--force] [--account <id>]")
	fmt.Println("  weixin-sdk send --account <id> --to <user> [--text <msg>] [--image <file>]")
	fmt.Println("  weixin-sdk monitor --account <id>")
	fmt.Println("  weixin-sdk list")
	fmt.Println("  weixin-sdk delete --account <id>")
}

func doLogin(sdk *weixinsdk.SDK, force bool, accountID string) {
	ctx := context.Background()

	fmt.Println("Starting QR code login...")
	if accountID != "" {
		fmt.Printf("Account ID: %s\n", accountID)
	}

	result, start, err := sdk.Login(ctx, accountID, force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		os.Exit(1)
	}

	if start != nil && start.QRCodeURL != "" {
		fmt.Println("\n请使用微信扫描以下二维码：")
		fmt.Println(start.QRCodeURL)
		fmt.Println()
	}

	if result.Connected {
		fmt.Printf("\n✅ 登录成功！\n")
		fmt.Printf("  账号 ID: %s\n", result.AccountID)
		fmt.Printf("  用户 ID: %s\n", result.UserID)
	} else {
		fmt.Printf("\n❌ 登录失败: %s\n", result.Message)
		os.Exit(1)
	}
}

func doSend(sdk *weixinsdk.SDK, accountID, to, text, image string) {
	if accountID == "" || to == "" {
		fmt.Fprintln(os.Stderr, "Error: --account and --to are required")
		os.Exit(1)
	}

	if text == "" && image == "" {
		fmt.Fprintln(os.Stderr, "Error: either --text or --image is required")
		os.Exit(1)
	}

	ctx := context.Background()

	// Get context token (if any)
	contextToken := sdk.GetContextToken(accountID, to)

	sender, err := sdk.NewSender(accountID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create sender: %v\n", err)
		os.Exit(1)
	}

	if text != "" {
		msgID, err := sender.SendText(ctx, to, text, contextToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send text: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Text message sent: %s\n", msgID)
	}

	if image != "" {
		data, err := os.ReadFile(image)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read image: %v\n", err)
			os.Exit(1)
		}
		msgID, err := sender.SendImage(ctx, to, "", data, contextToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send image: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Image message sent: %s\n", msgID)
	}
}

func doMonitor(sdk *weixinsdk.SDK, accountID string) {
	if accountID == "" {
		fmt.Fprintln(os.Stderr, "Error: --account is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nStopping monitor...")
		cancel()
	}()

	fmt.Printf("Starting monitor for account: %s\n", accountID)

	// Create message handler
	handler := func(ctx context.Context, msg *api.WeixinMessage, accID string) error {
		fmt.Printf("\n=== New Message ===\n")
		fmt.Printf("From: %s\n", msg.FromUserID)
		fmt.Printf("To: %s\n", msg.ToUserID)
		fmt.Printf("Time: %d\n", msg.CreateTimeMs)

		for _, item := range msg.ItemList {
			switch item.Type {
			case int(api.MessageItemTypeText):
				if item.TextItem != nil {
					fmt.Printf("Text: %s\n", item.TextItem.Text)
				}
			case int(api.MessageItemTypeImage):
				fmt.Println("Type: Image")
			case int(api.MessageItemTypeVoice):
				fmt.Println("Type: Voice")
				if item.VoiceItem != nil && item.VoiceItem.Text != "" {
					fmt.Printf("Transcription: %s\n", item.VoiceItem.Text)
				}
			case int(api.MessageItemTypeFile):
				fmt.Println("Type: File")
				if item.FileItem != nil {
					fmt.Printf("Filename: %s\n", item.FileItem.FileName)
				}
			case int(api.MessageItemTypeVideo):
				fmt.Println("Type: Video")
			}
		}

		// Store context token
		if msg.ContextToken != "" && msg.FromUserID != "" {
			sdk.SetContextToken(accID, msg.FromUserID, msg.ContextToken)
		}

		return nil
	}

	monitor, err := sdk.NewMonitor(accountID, weixinsdk.WithHandler(handler))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create monitor: %v\n", err)
		os.Exit(1)
	}

	if err := monitor.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Monitor error: %v\n", err)
	}
}

func doList(sdk *weixinsdk.SDK) {
	accounts := sdk.ListAccounts()
	if len(accounts) == 0 {
		fmt.Println("No accounts registered.")
		return
	}

	fmt.Println("Registered accounts:")
	for _, accID := range accounts {
		acc, err := sdk.GetAccount(accID)
		if err != nil {
			fmt.Printf("  %s (error: %v)\n", accID, err)
			continue
		}
		status := "not configured"
		if acc.Configured {
			status = "configured"
		}
		if acc.Enabled {
			status += ", enabled"
		}
		fmt.Printf("  %s (%s)\n", accID, status)
	}
}

func doDelete(sdk *weixinsdk.SDK, accountID string) {
	if accountID == "" {
		fmt.Fprintln(os.Stderr, "Error: --account is required")
		os.Exit(1)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Are you sure you want to delete account %s? (y/N): ", accountID)
	response, _ := reader.ReadString('\n')

	if response != "y\n" && response != "Y\n" {
		fmt.Println("Cancelled.")
		return
	}

	if err := sdk.DeleteAccount(accountID); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to delete account: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Account %s deleted.\n", accountID)
}
