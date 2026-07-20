package handlers

import (
	"encoding/json"
	"net/http"
	model "practice/internal/model"
	"strings"
)

const (
	contentTypeJSON = "application/json"
	maxBodyBytes    = 1 << 20 // 1 MiB — тело одной метрики никогда не больше
)

// decodeMetrics читает ровно один JSON-объект из тела запроса.
//
// json.Decoder, а не Unmarshal: не тянем тело целиком в память, и в инкременте 8
// тело будет обёрнуто в gzip.Reader — это поток.
// DisallowUnknownFields не включаем: в инкременте 14 придёт поле "hash".
func decodeMetrics(res http.ResponseWriter, req *http.Request) (model.Metrics, bool) {
	req.Body = http.MaxBytesReader(res, req.Body, maxBodyBytes)

	var in model.Metrics
	dec := json.NewDecoder(req.Body)
	if err := dec.Decode(&in); err != nil {
		// io.EOF (пустое тело), синтаксис, превышение лимита — всё это 400.
		http.Error(res, "invalid json body", http.StatusBadRequest)
		return in, false
	}
	// Decode останавливается после первого значения: '{...}мусор' прошёл бы молча.
	if dec.More() {
		http.Error(res, "unexpected trailing data after json object", http.StatusBadRequest)
		return in, false
	}

	in.ID = strings.TrimSpace(in.ID)
	return in, true
}

// writeJSON — единственная точка записи успешного JSON-ответа.
//
// Инварианты:
//  1. Header().Set строго ДО WriteHeader — после WriteHeader вызов Set() no-op.
//  2. Маршалим в буфер ДО WriteHeader: Encode(res) после WriteHeader(200) при
//     ошибке оставил бы клиенту 200 с обрезанным телом.
func writeJSON(res http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(res, "response encoding error", http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", contentTypeJSON)
	res.WriteHeader(status)
	res.Write(body)
}

// UpdateJSONHandler — POST /update и POST /update/.
// Тело — JSON, ответ — сохранённая метрика (для counter это накопленная delta).
func (h *Handler) UpdateJSONHandler(res http.ResponseWriter, req *http.Request) {
	in, ok := decodeMetrics(res, req)
	if !ok {
		return
	}

	// Пустое имя на текстовом маршруте даёт 404 (chi не матчит пустой сегмент).
	// Держим тот же контракт, чтобы поведение не разъезжалось.
	if in.ID == "" {
		http.Error(res, "metric id is required", http.StatusNotFound)
		return
	}

	m, err := in.ToDomain()
	if err != nil {
		// Неизвестный тип, отсутствующий delta/value — всё это 400.
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	_ = m.CalculateHash()

	if err := h.store.Save(m); err != nil {
		http.Error(res, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Save() возвращает только error, поэтому накопленное значение counter'а
	// перечитываем: MemStorage.Save мутирует лежащий в map *Metric
	// (existing.Delta += ...), а Get отдаёт копию этого объекта.
	stored, found := h.store.Get(in.Type(), m.ID)
	if !found || stored == nil {
		stored = m
	}

	writeJSON(res, http.StatusOK, model.FromDomain(stored))
}

// ValueJSONHandler — POST /value и POST /value/.
// В теле запроса заполнены id и type, в ответе — тот же JSON со значением.
func (h *Handler) ValueJSONHandler(res http.ResponseWriter, req *http.Request) {
	in, ok := decodeMetrics(res, req)
	if !ok {
		return
	}

	mtype := in.Type()
	if mtype == model.Unknown {
		http.Error(res, "Invalid metric type", http.StatusBadRequest)
		return
	}

	m, found := h.store.Get(mtype, in.ID)
	if !found {
		// http.Error сам ставит text/plain; смешивать с application/json нельзя.
		http.Error(res, "metric not found", http.StatusNotFound)
		return
	}

	writeJSON(res, http.StatusOK, model.FromDomain(m))
}
