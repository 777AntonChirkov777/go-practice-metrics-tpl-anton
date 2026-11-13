package handlers

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	model "practice/internal/model"
	"practice/internal/storage"

	"github.com/go-chi/chi/v5"
)

const maxPooledHTML = 64 << 10

var htmlBuffers = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

func putHTMLBuffer(buf *bytes.Buffer) {
	if buf.Cap() > maxPooledHTML {
		return
	}

	buf.Reset()
	htmlBuffers.Put(buf)
}

// Handler объединяет HTTP-обработчики метрик: текстовые, JSON и пакетные.
// Хранилище и приёмник аудита задаются в [NewHandler].
type Handler struct {
	store storage.MetricStorage
	audit Auditor
}

// NewHandler создаёт [Handler] поверх хранилища store и приёмника аудита
// auditor. Оба аргумента обязательны: хендлеры обращаются к ним без проверки
// на nil.
func NewHandler(store storage.MetricStorage, auditor Auditor) *Handler {
	return &Handler{store: store, audit: auditor}
}

// UpdateHandler обрабатывает POST /update/{type}/{name}/{value}: сохраняет одно
// значение метрики, переданное сегментами пути.
//
// Тип (gauge или counter) сравнивается без учёта регистра, name и value
// очищаются от пробелов по краям. Для gauge значение разбирается как float64 и
// заменяет прежнее, для counter — как десятичное int64 и прибавляется к
// накопленному. Тело запроса не читается, тело успешного ответа пустое.
//
// Коды ответа:
//   - 200 — метрика сохранена;
//   - 400 — тип пуст или имя либо значение пусты после обрезки пробелов
//     ("Expected /update/{type}/{name}/{value}"),
//     неизвестный тип ("Invalid metric type") или значение не разбирается как
//     число нужного типа ("Invalid metric value");
//   - 500 — хранилище вернуло ошибку ("storage error: ...").
//
// При включённом аудите после сохранения вызывается [Auditor.Record] с именем
// метрики и IP-адресом клиента.
func (h *Handler) UpdateHandler(res http.ResponseWriter, req *http.Request) {
	typeStr := chi.URLParam(req, "type")
	name := strings.TrimSpace(chi.URLParam(req, "name"))
	valueStr := strings.TrimSpace(chi.URLParam(req, "value"))

	if typeStr == "" || name == "" || valueStr == "" {
		http.Error(res, "Expected /update/{type}/{name}/{value}", http.StatusBadRequest)
		return
	}

	mtype := model.GetTypeMetric(typeStr)
	if mtype == model.Unknown {
		http.Error(res, "Invalid metric type", http.StatusBadRequest)
		return
	}
	if !model.IsParseMetricValue(mtype, valueStr) {
		http.Error(res, "Invalid metric value", http.StatusBadRequest)
		return
	}

	var m *model.Metric
	switch mtype {
	case model.Gauge:
		v, _ := model.ParseGauge(mtype, valueStr)
		m = model.NewGaugeMetric(name, v)
	case model.Counter:
		v, _ := model.ParseCounter(mtype, valueStr)
		m = model.NewCountMetric(name, v)
	}

	if err := h.store.Save(req.Context(), m); err != nil {
		http.Error(res, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.audit.Enabled() {
		h.audit.Record([]string{name}, clientIP(req))
	}

	res.WriteHeader(http.StatusOK)
}

// ValueHandler обрабатывает GET /value/{type}/{name}: возвращает текущее
// значение метрики в виде текста.
//
// Тип сравнивается без учёта регистра, name очищается от пробелов по краям.
// Ответ приходит с Content-Type "text/plain; charset=utf-8": для gauge тело
// содержит значение, отформатированное [strconv.FormatFloat] с форматом 'f' и
// точностью -1 (без экспоненты и лишних нулей), для counter — накопленное
// целое.
//
// Коды ответа:
//   - 200 — метрика найдена, значение в теле;
//   - 400 — неизвестный тип ("Invalid metric type");
//   - 404 — метрики нет в хранилище ("metric not found");
//   - 500 — иная ошибка хранилища ("storage error: ...").
func (h *Handler) ValueHandler(res http.ResponseWriter, req *http.Request) {
	typeStr := chi.URLParam(req, "type")
	name := strings.TrimSpace(chi.URLParam(req, "name"))

	mtype := model.GetTypeMetric(typeStr)
	if mtype == model.Unknown {
		http.Error(res, "Invalid metric type", http.StatusBadRequest)
		return
	}

	m, err := h.store.Get(req.Context(), mtype, name)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(res, "metric not found", http.StatusNotFound)
			return
		}
		http.Error(res, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", "text/plain; charset=utf-8")
	res.WriteHeader(http.StatusOK)

	switch mtype {
	case model.Gauge:
		res.Write([]byte(strconv.FormatFloat(m.Value, 'f', -1, 64)))
	case model.Counter:
		res.Write([]byte(strconv.FormatInt(m.Delta, 10)))
	}
}

// ListHandler обрабатывает GET /: отдаёт HTML-страницу со списком всех метрик
// из хранилища.
//
// Каждая метрика выводится элементом списка в виде "имя [тип] = значение", для
// пустого хранилища — строкой "no metrics yet". Ответ приходит с Content-Type
// "text/html; charset=utf-8". Имена метрик в разметку выводятся как есть, без
// экранирования.
//
// Коды ответа:
//   - 200 — страница сформирована;
//   - 500 — хранилище вернуло ошибку ("storage error: ...").
func (h *Handler) ListHandler(res http.ResponseWriter, req *http.Request) {
	all, err := h.store.GetAll(req.Context())
	if err != nil {
		http.Error(res, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	buf := htmlBuffers.Get().(*bytes.Buffer)
	defer putHTMLBuffer(buf)

	buf.Reset()
	buf.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Metrics</title></head><body>`)
	buf.WriteString("<h1>Metrics</h1><ul>")
	if len(all) == 0 {
		buf.WriteString("<li>no metrics yet</li>")
	}

	var scratch [32]byte
	for _, m := range all {
		buf.WriteString("<li><b>")
		buf.WriteString(m.ID)
		buf.WriteString("</b> [")
		buf.WriteString(model.MetricType(m.MType).String())
		buf.WriteString("] = ")

		if m.MType == int8(model.Gauge) {
			buf.Write(strconv.AppendFloat(scratch[:0], m.Value, 'f', -1, 64))
		} else {
			buf.Write(strconv.AppendInt(scratch[:0], m.Delta, 10))
		}

		buf.WriteString("</li>")
	}
	buf.WriteString("</ul></body></html>")

	res.Header().Set("Content-Type", "text/html; charset=utf-8")
	res.WriteHeader(http.StatusOK)
	res.Write(buf.Bytes())
}
