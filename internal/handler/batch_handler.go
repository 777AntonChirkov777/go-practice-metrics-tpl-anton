package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	model "practice/internal/model"
)

const maxBatchBodyBytes = 4 << 20

// UpdatesJSONHandler обрабатывает POST /updates и POST /updates/: сохраняет
// пакет метрик, переданный JSON-массивом.
//
// Тело запроса — массив объектов [model.Metrics] с полями id, type, а также
// delta для counter или value для gauge. Размер тела ограничен 4 МиБ. Пробелы по
// краям id отбрасываются. Все элементы проверяются до сохранения, поэтому при
// ошибке в любом из них ничего не сохраняется. Валидный пакет записывается через
// SaveBatch за один вызов: gauge заменяет значение, counter накапливается. Пустой
// массив принимается без обращения к хранилищу. Тело успешного ответа пустое.
//
// Коды ответа:
//   - 200 — пакет сохранён (или пуст);
//   - 400 — тело не разбирается как JSON-массив или превышает лимит ("invalid json
//     body"), после массива [json.Decoder.More] находит ещё данные ("unexpected
//     trailing data after json array"), у элемента пустой id ("metric id is
//     required") либо элемент не превращается в метрику (текст ошибки
//     [model.Metrics.ToDomain]);
//   - 500 — хранилище вернуло ошибку ("storage error: ...").
//
// При включённом аудите после сохранения вызывается [Auditor.Record] с именами
// метрик пакета и IP-адресом клиента.
func (h *Handler) UpdatesJSONHandler(res http.ResponseWriter, req *http.Request) {
	req.Body = http.MaxBytesReader(res, req.Body, maxBatchBodyBytes)

	var in []model.Metrics
	dec := json.NewDecoder(req.Body)
	if err := dec.Decode(&in); err != nil {
		http.Error(res, "invalid json body", http.StatusBadRequest)
		return
	}
	if dec.More() {
		http.Error(res, "unexpected trailing data after json array", http.StatusBadRequest)
		return
	}

	if len(in) == 0 {
		res.WriteHeader(http.StatusOK)
		return
	}

	batch := make([]*model.Metric, 0, len(in))

	audited := h.audit.Enabled()

	var names []string
	if audited {
		names = make([]string, 0, len(in))
	}

	for _, w := range in {
		w.ID = strings.TrimSpace(w.ID)
		if w.ID == "" {
			http.Error(res, "metric id is required", http.StatusBadRequest)
			return
		}

		m, err := w.ToDomain()
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}

		batch = append(batch, m)

		if audited {
			names = append(names, m.ID)
		}
	}

	if err := h.store.SaveBatch(req.Context(), batch); err != nil {
		http.Error(res, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if audited {
		h.audit.Record(names, clientIP(req))
	}

	res.WriteHeader(http.StatusOK)
}
