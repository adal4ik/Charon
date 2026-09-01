# Charon

Простой Ledger-сервис на Go для работы с виртуальными счетами.

Поддерживает:

* создание счёта;
* получение текущего баланса;
* пополнение счёта;
* перевод средств между счетами;
* корректную обработку конкурентных списаний.

## Стек

* go 1.26.4
* PostgreSQL 17
* chi v5
* pgx v5
* Docker Compose

## Запуск

Для запуска PostgreSQL и приложения:

```bash
docker compose up --build
```

После запуска API доступно по адресу:

```text
http://localhost:8080
```

PostgreSQL запускается в отдельном контейнере. Приложение стартует после успешного healthcheck базы данных.

Схема базы создаётся автоматически при первом запуске контейнера PostgreSQL.

Если порт `8080` занят:

```bash
HTTP_PORT=18080 docker compose up --build
```

Тогда API будет доступно по адресу:

```text
http://localhost:18080
```

## API

### Создать счёт

```bash
curl -X POST http://localhost:8080/accounts
```

Пример ответа:

```json
{"id":1,"balance":0}
```

### Получить счёт

```bash
curl http://localhost:8080/accounts/1
```

### Пополнить счёт

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"amount":100}' \
  http://localhost:8080/accounts/1/deposit
```

### Перевести средства

Сначала создайте второй счёт, затем:

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"from_account_id":1,"to_account_id":2,"amount":60}' \
  http://localhost:8080/transfers
```

При успешном переводе возвращается:

```text
204 No Content
```

## Конкурентное списание

Перевод выполняется внутри PostgreSQL-транзакции.

Для счетов используется:

```sql
SELECT id, balance
FROM accounts
WHERE id IN ($1, $2)
ORDER BY id
FOR UPDATE;
```

Это блокирует строки счетов до завершения транзакции.

Например, если баланс отправителя равен `100` и одновременно выполняются два перевода по `60`, успешно завершится только один. Второй будет отклонён из-за недостаточного баланса, а итоговый баланс отправителя будет `40`.

Дополнительно отрицательный баланс запрещён на уровне PostgreSQL:

```sql
CHECK (balance >= 0)
```

## Тесты

Обычные тесты:

```bash
go test ./...
```

Проверка Go data races:

```bash
go test -race ./...
```

Для concurrency integration test нужен запущенный PostgreSQL:

```bash
TEST_DATABASE_URL='postgres://charon:charon@localhost:5432/charon?sslmode=disable' \
go test -count=1 -v ./internal/repository \
-run '^TestPostgresRepositoryTransferConcurrent$'
```

Интеграционный тест запускает два конкурентных перевода по `60` со счёта с балансом `100` и проверяет, что:

* один перевод завершается успешно;
* второй возвращает `insufficient funds`;
* баланс отправителя становится `40`;
* отрицательного баланса не возникает.

## Остановка

Остановить приложение и PostgreSQL:

```bash
docker compose down
```

Удалить также локальный PostgreSQL volume:

```bash
docker compose down -v
```

После удаления volume схема базы будет создана заново при следующем запуске.
