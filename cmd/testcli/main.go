// Test CLI providers
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jxmullins/thekanbansociety/internal/provider"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("Testing CLI providers...")

	providers := provider.DetectCLIProviders()
	fmt.Printf("Found %d CLI providers\n", len(providers))

	for _, p := range providers {
		fmt.Printf("\n=== Testing %s ===\n", p.Name())

		resp, err := p.Invoke(ctx, provider.Request{
			Prompt: "Say hello in exactly one word. Just the word, nothing else.",
		})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Response: %q\n", resp.Content)
		}
	}

	fmt.Println("\nDone!")
}
