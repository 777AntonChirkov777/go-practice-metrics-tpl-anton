// Package handlers содержит HTTP-обработчики сервиса сбора метрик.
//
// Основной тип — [Handler], создаваемый через [NewHandler] поверх хранилища
// storage.MetricStorage и приёмника аудита [Auditor]. Его методы реализуют три
// группы эндпоинтов:
//   - текстовые: [Handler.UpdateHandler] (POST /update/{type}/{name}/{value}),
//     [Handler.ValueHandler] (GET /value/{type}/{name}) и [Handler.ListHandler]
//     (GET /, HTML-страница со всеми метриками);
//   - JSON для одной метрики: [Handler.UpdateJSONHandler] (POST /update) и
//     [Handler.ValueJSONHandler] (POST /value);
//   - пакетный JSON: [Handler.UpdatesJSONHandler] (POST /updates).
//
// Проверку доступности базы данных выполняет [PingHandler] (GET /ping),
// создаваемый через [NewPingHandler] и использующий интерфейс [Pinger].
//
// Gauge при сохранении заменяет прежнее значение, counter накапливается. Если
// аудит включён, после успешного сохранения хендлеры передают в [Auditor] имена
// метрик и IP-адрес клиента.
//
// Пакет не регистрирует маршруты сам: это делает пакет server, где методы
// подключаются к маршрутизатору chi вместе с middleware логирования, сжатия и
// подписи.
package handlers
