package logging

import (
    "fmt"
    "time"
)

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

func Warn(msg string, kv ...any) { Info("WARN: "+msg, kv...) }
func Err(msg string, kv ...any)  { Info("ERR: "+msg, kv...) }
