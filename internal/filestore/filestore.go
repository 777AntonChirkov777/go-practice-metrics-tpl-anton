package filestore

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	model "practice/internal/model"
)

type Snapshotter interface {
	GetAll(ctx context.Context) ([]model.Metric, error)
}

type FileStore struct {
	mu       sync.Mutex
	path     string
	dirReady bool
}

func New(path string) *FileStore {
	return &FileStore{path: path}
}

const dumpBufferSize = 32 << 10

type dumpEncoder struct {
	buf *bufio.Writer
	enc *json.Encoder
}

var dumpEncoders = sync.Pool{
	New: func() any {
		d := &dumpEncoder{buf: bufio.NewWriterSize(nil, dumpBufferSize)}
		d.enc = json.NewEncoder(d.buf)
		d.enc.SetIndent("", "  ")
		return d
	},
}

func writeDump(path string, dtos []model.Metrics) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	d := dumpEncoders.Get().(*dumpEncoder)
	defer func() {
		d.buf.Reset(nil)
		dumpEncoders.Put(d)
	}()

	d.buf.Reset(f)

	if err := d.enc.Encode(dtos); err != nil {
		_ = f.Close()
		return err
	}
	if err := d.buf.Flush(); err != nil {
		_ = f.Close()
		return err
	}

	return f.Close()
}

func (fs *FileStore) SaveFrom(ctx context.Context, src Snapshotter) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	metrics, err := src.GetAll(ctx)
	if err != nil {
		return err
	}

	return fs.saveLocked(metrics)
}

func (fs *FileStore) save(metrics []model.Metric) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.saveLocked(metrics)
}

func (fs *FileStore) saveLocked(metrics []model.Metric) error {
	if fs.path == "" {
		return nil
	}

	dtos := make([]model.Metrics, 0, len(metrics))
	for i := range metrics {
		dtos = append(dtos, model.FromDomain(&metrics[i]))
	}

	if !fs.dirReady {
		if dir := filepath.Dir(fs.path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
		fs.dirReady = true
	}

	// Пишем во временный файл рядом с целевым и атомарно переименовываем,
	// чтобы читатель никогда не увидел частично записанный файл.
	tmp := fs.path + ".tmp"
	if err := writeDump(tmp, dtos); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, fs.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Load читает метрики из файла. Отсутствие файла или пустой путь — не ошибка:
// возвращается пустой срез (сохранять ещё нечего).
func (fs *FileStore) Load() ([]*model.Metric, error) {
	if fs.path == "" {
		return nil, nil
	}

	fs.mu.Lock()
	data, err := os.ReadFile(fs.path)
	fs.mu.Unlock()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	var dtos []model.Metrics
	if err := json.Unmarshal(data, &dtos); err != nil {
		return nil, err
	}

	out := make([]*model.Metric, 0, len(dtos))
	for _, d := range dtos {
		m, err := d.ToDomain()
		if err != nil {
			// Битую/неполную запись пропускаем, остальные восстанавливаем.
			continue
		}
		out = append(out, m)
	}
	return out, nil
}
