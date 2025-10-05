package core

import "time"

type SourceMeta struct {
    Kind string `json:"kind"`
    Port string `json:"port,omitempty"`
    Baud int    `json:"baud,omitempty"`
    Addr string `json:"addr,omitempty"`
    Path string `json:"path,omitempty"`
}

type Run struct {
    ID      string     `json:"run_id"`
    Source  SourceMeta `json:"source"`
    Started time.Time  `json:"started"`
}

type Record struct {
    TS      time.Time      `json:"ts"`
    Seq     uint64         `json:"seq"`
    Type    string         `json:"type"`
    Payload map[string]any `json:"payload"`
}
