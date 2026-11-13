# Гофермарт

HTTP API накопительной системы лояльности: регистрация и вход, загрузка заказов, начисление и списание баллов, просмотр баланса и истории списаний.

## Требования

- Go 1.27.1 или новее.
- Docker с Docker Compose для локальной PostgreSQL.
- Сервис начислений accrual из [официального шаблона](https://github.com/yandex-praktikum/go-musthave-diploma-tpl/tree/master/cmd/accrual).

Все команды выполняются из корня репозитория.

## База данных

```sh
docker compose up -d --wait postgres
```

Строка подключения к локальной PostgreSQL:

```text
postgres://gofermart:password@localhost:5434/gofermart?sslmode=disable
```

## Конфигурация

| Переменная окружения | Флаг | Значение |
| --- | --- | --- |
| `RUN_ADDRESS` | `-a` | Адрес HTTP-сервера, по умолчанию `:8080` |
| `DATABASE_URI` | `-d` | Строка подключения к PostgreSQL, обязательна |
| `ACCRUAL_SYSTEM_ADDRESS` | `-r` | URL сервиса начислений, обязателен, например `http://localhost:8081` |
| `JWT_SECRET` | - | Ключ подписи JWT длиной не менее 32 байт |

Флаги имеют приоритет над переменными окружения.

## Сборка и локальный запуск

### Windows amd64 (PowerShell)

Терминал 1: accrual.

```powershell
$accrualBinary = Join-Path $env:TEMP 'gofermart-accrual.exe'
Invoke-WebRequest 'https://raw.githubusercontent.com/yandex-praktikum/go-musthave-diploma-tpl/c9b1ea58dd01fba3c93b69a8dd01786c30ce3e26/cmd/accrual/accrual_windows_amd64' -OutFile $accrualBinary
$env:RUN_ADDRESS = 'localhost:8081'
$env:DATABASE_URI = 'postgres://gofermart:password@localhost:5434/gofermart?sslmode=disable'
& $accrualBinary
```

Терминал 2: Гофермарт.

```powershell
$env:RUN_ADDRESS = 'localhost:8080'
$env:DATABASE_URI = 'postgres://gofermart:password@localhost:5434/gofermart?sslmode=disable'
$env:ACCRUAL_SYSTEM_ADDRESS = 'http://localhost:8081'
$env:JWT_SECRET = 'local-development-secret-at-least-32-bytes'
go build -o gofermart.exe ./cmd/gofermart
.\gofermart.exe
```

### Linux amd64

Терминал 1: accrual.

```sh
curl -fsSL 'https://raw.githubusercontent.com/yandex-praktikum/go-musthave-diploma-tpl/c9b1ea58dd01fba3c93b69a8dd01786c30ce3e26/cmd/accrual/accrual_linux_amd64' -o /tmp/gofermart-accrual
chmod +x /tmp/gofermart-accrual
RUN_ADDRESS=localhost:8081 \
DATABASE_URI='postgres://gofermart:password@localhost:5434/gofermart?sslmode=disable' \
/tmp/gofermart-accrual
```

Терминал 2: Гофермарт.

```sh
go build -o gofermart ./cmd/gofermart
RUN_ADDRESS=localhost:8080 \
DATABASE_URI='postgres://gofermart:password@localhost:5434/gofermart?sslmode=disable' \
ACCRUAL_SYSTEM_ADDRESS=http://localhost:8081 \
JWT_SECRET='local-development-secret-at-least-32-bytes' \
./gofermart
```

## Миграции

При старте сервиса Goose автоматически применяет ещё не выполненные SQL-миграции из `migrations/`.

## Взаимодействие сервисов

Гофермарт принимает номер заказа через `POST /api/user/orders`, сохраняет его в PostgreSQL и в фоне запрашивает результат расчёта у accrual через `GET /api/orders/{number}`. Полученные начисления записываются в БД и учитываются в балансе пользователя.

## Проверки

```sh
go test ./...
```

Для теста миграций задайте строку подключения к тестовой БД в `TEST_DATABASE_URI`.

Для pull request GitHub Actions запускает официальные функциональные автотесты и `statictest`.
