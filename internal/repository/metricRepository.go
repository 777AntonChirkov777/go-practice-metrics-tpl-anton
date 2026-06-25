package repositories

import (
	"encoding/json"
	"fmt"
	"os"
	models "practice/internal/model"
)

func SaveMetric(newMetric models.Metric) error {
	tmpFile := models.FileName + ".tmp"

	// Открываем исходный файл
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

	// Копируем старые записи, заменяя нужную
	if inFile != nil {
		decoder := json.NewDecoder(inFile)
		for decoder.More() {
			var m models.Metric
			if err := decoder.Decode(&m); err != nil {
				continue
			}
			if m.ID == newMetric.ID && m.MType == newMetric.MType {
				m = newMetric
				exists = true
			}
			if err := encoder.Encode(m); err != nil {
				outFile.Close()
				inFile.Close()
				os.Remove(tmpFile)
				return err
			}
		}
		// Явно закрываем исходный файл перед переименованием
		if err := inFile.Close(); err != nil {
			outFile.Close()
			os.Remove(tmpFile)
			return err
		}
	}

	// Если метрика не найдена, добавляем новую
	if !exists {
		if err := encoder.Encode(newMetric); err != nil {
			outFile.Close()
			os.Remove(tmpFile)
			return err
		}
	}

	// Закрываем временный файл перед переименованием
	if err := outFile.Close(); err != nil {
		os.Remove(tmpFile)
		return err
	}

	// Атомарная замена на Windows/Linux
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
