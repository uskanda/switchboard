package claudecode

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
)

// Progress follows the stream-json output of a run while it goes on. It keeps how many times each tool was called
// and the latest text of the assistant, and nothing of what the tools were given or returned.
type Progress struct {
	path string
	off  int64 // where the next read starts: just after the last complete line
	// Tools counts the tool_use blocks by tool name.
	Tools map[string]int
	// Text is the latest text block of an assistant message.
	Text string
}

// NewProgress follows the output file at path (RunDir/stream.jsonl), which may not exist yet.
func NewProgress(path string) *Progress { return &Progress{path: path, Tools: map[string]int{}} }

// Read takes in the lines completed since the last read. A last line still being written is left for the next one.
func (p *Progress) Read() error {
	f, err := os.Open(p.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(p.off, io.SeekStart); err != nil {
		return err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	end := bytes.LastIndexByte(data, '\n')
	if end < 0 {
		return nil
	}
	for _, line := range bytes.Split(data[:end], []byte("\n")) {
		p.take(line)
	}
	p.off += int64(end + 1)
	return nil
}

func (p *Progress) take(line []byte) {
	var m struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
				Name string `json:"name"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &m) != nil || m.Type != "assistant" {
		return
	}
	for _, c := range m.Message.Content {
		switch c.Type {
		case "tool_use":
			if c.Name != "" {
				p.Tools[c.Name]++
			}
		case "text":
			if c.Text != "" {
				p.Text = c.Text
			}
		}
	}
}
