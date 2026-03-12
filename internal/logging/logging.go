// Package logging provides lightweight structured logging helpers that write
// timestamped key-value lines to stdout.
package logging

import (
	"fmt"
	"time"
)

// Info logs an informational message with optional key-value pairs to stdout.
func Info(msg string, kv ...any) {
	fmt.Printf("[%s] %s", time.Now().Format(time.RFC3339), msg)
	if len(kv) > 0 {
		fmt.Print(" ")
		for i := 0; i < len(kv); i += 2 {
			if i+1 < len(kv) {
				fmt.Printf("%v=%v ", kv[i], kv[i+1])
			}
		}
	}
	fmt.Println()
}

// Warn logs a warning message with optional key-value pairs to stdout.
func Warn(msg string, kv ...any) { Info("WARN: "+msg, kv...) }

// Err logs an error message with optional key-value pairs to stdout.
func Err(msg string, kv ...any) { Info("ERR: "+msg, kv...) }
