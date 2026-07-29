package filestore

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	model "practice/internal/model"
)

type Snapshotter interface {
	GetAll() []*model.Metric
}

type FileStore struct {
	mu       sync.Mutex
	path     string
	dirReady bool
}

func New(path string) *FileStore {
	return &FileStore{path: path}
}

func (fs *FileStore) SaveFrom(src Snapshotter) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.saveLocked(src.GetAll())
}

func (fs *FileStore) save(metrics []*model.Metric) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.saveLocked(metrics)
}

func (fs *FileStore) saveLocked(metrics []*model.Metric) error {
	if fs.path == "" {
		return nil
	}

	dtos := make([]model.Metrics, 0, len(metrics))
	for _, m := range metrics {
		dtos = append(dtos, model.FromDomain(m))
	}

	data, err := json.MarshalIndent(dtos, "", "  ")
	if err != nil {
		return err
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
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
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
