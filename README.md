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

## Шифрование агент → сервер

Агент может шифровать отчёты публичным ключом сервера, а сервер — расшифровывать их своим приватным ключом. Путь к файлу ключа задаётся флагом `-crypto-key` или переменной окружения `CRYPTO_KEY`; переменная окружения важнее флага. Агенту передаётся публичный ключ, серверу — приватный. Пустой путь выключает шифрование.

Пара ключей создаётся через openssl, он есть, например, в Git Bash:

```bash
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:4096 -out private.pem
openssl pkey -in private.pem -pubout -out public.pem
```

Принимаются ключи RSA длиной не меньше 2048 бит в формате PEM:

- публичный ключ — блок `PUBLIC KEY` (PKIX) или `RSA PUBLIC KEY` (PKCS#1);
- приватный ключ — блок `PRIVATE KEY` (PKCS#8) или `RSA PRIVATE KEY` (PKCS#1), без пароля.

Файлы `*.pem` перечислены в `.gitignore`, чтобы ключи не попали в репозиторий. Если путь задан, но ключ не загрузился, процесс завершается с кодом 1 и пишет причину в stderr.

Запуск:

```bash
./cmd/server/server -crypto-key=private.pem
./cmd/agent/agent -crypto-key=public.pem
```

То же через окружение: `CRYPTO_KEY=private.pem ./cmd/server/server` и `CRYPTO_KEY=public.pem ./cmd/agent/agent`.

Как шифруется отчёт:

1. Агент сериализует батч в JSON и подписывает его, если задан ключ `-k`.
2. Затем сжимает тело gzip.
3. Затем шифрует его гибридной схемой: на каждый запрос создаётся случайный ключ AES-256, тело шифруется AES-256-GCM, а ключ AES шифруется RSA-OAEP с SHA-256. Тело запроса — зашифрованный ключ AES, за ним nonce длиной 12 байт, за ним шифротекст с тегом.
4. Запрос помечается заголовком `X-Encryption: rsa-oaep-aes256gcm`. Заголовки `Content-Encoding: gzip` и `HashSHA256` описывают тело после расшифровки.

Как сервер обрабатывает запросы:

- С приватным ключом сервер расшифровывает запросы с заголовком `X-Encryption`, затем распаковывает их и проверяет подпись как обычно. Запрос с непустым телом без этого заголовка получает 400. Запросы без тела (`GET /`, `GET /value/...`, `GET /ping`, `POST /update/{type}/{name}/{value}`) работают как раньше.
- Без приватного ключа сервер отвечает 400 на любой запрос с заголовком `X-Encryption`, остальные обрабатывает как раньше.
- Ответы сервера не шифруются.

Публичный ключ не секретен: зашифровать запрос может кто угодно. Подлинность отправителя подтверждает только подпись `-k`.

Включать шифрование нужно на обеих сторонах одновременно. Иначе сервер с ключом отклоняет открытые отчёты, а сервер без ключа — зашифрованные.

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
