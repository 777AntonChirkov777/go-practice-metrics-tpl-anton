package repositories

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	models "practice/internal/model"
	"strings"
)

func SaveMetric(newMetric models.Metric) error {
	tmpFile := models.FileName + ".tmp"

	// Открываем исходный файл (если он существует)
	inFile, err := os.Open(models.FileName)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Создаём временный файл
	outFile, err := os.Create(tmpFile)
	if err != nil {
		if inFile != nil {
			inFile.Close()
		}
		return err
	}

	encoder := json.NewEncoder(outFile)
	var exists bool

	// Если исходный файл открыт, обрабатываем его построчно
	if inFile != nil {
		scanner := bufio.NewScanner(inFile)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue // пропускаем пустые строки
			}

			var m models.Metric
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				continue // пропускаем некорректные JSON-строки
			}

			// Если нашли метрику с таким же ID и MType – заменяем её новой
			if m.ID == newMetric.ID && m.MType == newMetric.MType {
				m = newMetric
				exists = true
			}

			// Перезаписываем метрику во временный файл
			if err := encoder.Encode(m); err != nil {
				outFile.Close()
				inFile.Close()
				os.Remove(tmpFile)
				return err
			}
		}

		// Проверяем ошибки сканера
		if err := scanner.Err(); err != nil {
			outFile.Close()
			inFile.Close()
			os.Remove(tmpFile)
			return err
		}

		// Закрываем исходный файл
		if err := inFile.Close(); err != nil {
			outFile.Close()
			os.Remove(tmpFile)
			return err
		}
	}

	// Если метрика не была найдена в старом файле – дописываем её
	if !exists {
		if err := encoder.Encode(newMetric); err != nil {
			outFile.Close()
			os.Remove(tmpFile)
			return err
		}
	}

	// Закрываем временный файл
	if err := outFile.Close(); err != nil {
		os.Remove(tmpFile)
		return err
	}

	// Атомарно заменяем старый файл новым
	if err := os.Rename(tmpFile, models.FileName); err != nil {
		os.Remove(tmpFile)
		return err
	}

	return nil
}

func FindMetric(targetID string, targetMType int8) (*models.Metric, error) {
	file, err := os.OpenFile(models.FileName, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	var found *models.Metric
	var count int

	for decoder.More() {
		var m models.Metric
		if err := decoder.Decode(&m); err != nil {
			continue // битая строка
		}
		if m.ID == targetID && m.MType == targetMType {
			count++
			if count > 1 {
				panic(fmt.Errorf("duplicate metric: ID=%s MType=%d", targetID, targetMType))
			}
			mCopy := m
			found = &mCopy
		}
	}
	return found, nil
}

// ReadAll возвращает все метрики из файла.
func ReadAll() ([]*models.Metric, error) {
	file, err := os.OpenFile(models.FileName, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var result []*models.Metric
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var m models.Metric
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		mCopy := m
		result = append(result, &mCopy)
	}
	return result, scanner.Err()
}
