# go-musthave-metrics-tpl

Шаблон репозитория для трека «Сервер сбора метрик и алертинга».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-metrics-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Сборка с информацией о версии

При старте сервер и агент первыми тремя строками выводят в stdout информацию о сборке:

```
Build version: v1.0.0
Build date: 2026-11-23T10:00:00Z
Build commit: 1ffabbc
```

Значения задаются при сборке флагом линковщика `-X` в переменные `main.buildVersion`, `main.buildDate` и `main.buildCommit`. Если значение не задано или пустое, вместо него выводится `N/A`, например при `go run ./cmd/server` или при сборке без `-ldflags`.

Сборка в bash:

```bash
VERSION=v1.0.0
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT=$(git rev-parse --short HEAD)
LDFLAGS="-X main.buildVersion=$VERSION -X main.buildDate=$DATE -X main.buildCommit=$COMMIT"
go build -ldflags "$LDFLAGS" -o cmd/server/server ./cmd/server
go build -ldflags "$LDFLAGS" -o cmd/agent/agent ./cmd/agent
```

Сборка в PowerShell:

```powershell
$version = 'v1.0.0'
$date = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
$commit = git rev-parse --short HEAD
$ldflags = "-X main.buildVersion=$version -X main.buildDate=$date -X main.buildCommit=$commit"
go build -ldflags $ldflags -o cmd/server/server.exe ./cmd/server
go build -ldflags $ldflags -o cmd/agent/agent.exe ./cmd/agent
```

Дата в формате ISO 8601 (UTC) пробелов не содержит, поэтому кавычки внутри значения `-ldflags` не нужны. Значение с пробелами берётся в одинарные кавычки внутри `-ldflags`, например `-X 'main.buildDate=2026-11-23 10:00:00'`.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**
