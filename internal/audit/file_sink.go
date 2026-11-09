package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type FileSink struct {
	file *os.File
}

func NewFileSink(path string) (*FileSink, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть файл аудита %q: %w", path, err)
	}

	return &FileSink{file: file}, nil
}

func (s *FileSink) Notify(_ context.Context, e Event) error {
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	_, err = s.file.Write(line)
	return err
}

func (s *FileSink) Close() error {
	return s.file.Close()
}
