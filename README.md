# bank

Игрушечный банк на Go: регистрация, счета и переводы между ними. PostgreSQL, templ, htmx.

## Устройство

Модульный монолит. Каждая фича в `features/` разделена на `domain`, `application`, `infrastructure` и `adapters`, собираются они в `main.go` и `configurator.go`.

- `user_service` — пользователи, пароли (argon2id), JWT
- `accounts` — счета
- `ledger` — журнал переводов
- `website` — интерфейс на templ и htmx

## Запуск

Нужны Go, Docker, make, [migrate](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate), [templ](https://templ.guide) и [wgo](https://github.com/bokwoon95/wgo).

```bash
cp .env.example .env
make db-up
make run-debug
```

Приложение: http://localhost:8085

## Тесты

```bash
make test-db-up
go test ./...
```

Каждый тест с базой работает в своей схеме. Без базы: `go test -short ./...`.

## Казна

Все деньги изначально лежат на счёте пользователя `treasury`. Пароль из `.env.example`: `VKqmGVFT6LgsJRLrMEc9kVuBK5rZCg4U`.

Задаётся он хешем `treasury_user_hash` в `db/migrations/000005_add_treasury_account.up.sql`. Хеш получен текущей реализацией argon2id из `features/user_service/infrastucture/passwordhasher`, новый нужно делать ей же.

## TODO

- Защита от CSRF.
- `exp` у JWT: сейчас токен бессрочный.
- Индексы на `transaction(debit_account_id)` и `transaction(credit_account_id)`: баланс считается полным проходом по журналу.
- `GET /accounts` создаёт счёт, если его нет: побочный эффект на GET, параллельные первые запросы создают несколько счетов.

## NOTES

Пример обработки запроса из удалённого `features/user_service/adapters/http`. Роутер `private` обёрнут в `auth.Middleware` (см. `main.go`), данные из JWT обработчик достаёт сам через `auth.Authenticate`, JSON-тело разбирает `pkg/jsonhelp`.

```go
private.HandleFunc("/user/role", func(w http.ResponseWriter, r *http.Request) {
	auth, err := auth.Authenticate(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("auth error: %s", err), http.StatusInternalServerError)
		return
	}

	if auth.Role != roles.RoleAdmin {
		http.Error(w, "aborted", http.StatusUnauthorized)
		return
	}

	jsonhelp.HandleWithBody(w, r, func(ctx context.Context, r *changerole.Request) (*changerole.Response, error) {
		r.RequesterId = auth.UserId
		return changerole.Handle(ctx, app, r)
	})
}).Methods("PUT")
```
