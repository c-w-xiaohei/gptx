package cli

import (
	"fmt"
	"os"
	"strings"
)

func promptWithContextFiles(prompt string, paths []string) (string, error) {
	if len(paths) == 0 {
		return prompt, nil
	}
	var b strings.Builder
	b.WriteString(prompt)
	b.WriteString("\n\nAdditional context files:")
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read context file %q: %w", path, err)
		}
		b.WriteString("\n--- BEGIN CONTEXT FILE: ")
		b.WriteString(path)
		b.WriteString(" ---\n")
		b.Write(data)
		if len(data) == 0 || data[len(data)-1] != '\n' {
			b.WriteByte('\n')
		}
		b.WriteString("--- END CONTEXT FILE: ")
		b.WriteString(path)
		b.WriteString(" ---")
	}
	return b.String(), nil
}
