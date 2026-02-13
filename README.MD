# Withdraw API

Backend service untuk autentikasi user wallet, cek saldo, dan withdrawal dengan idempotency dan rate limiting.

## Ringkas

- Bahasa: Go
- HTTP framework: Fiber v3
- Dependency injection: Uber Fx
- Database: PostgreSQL (sqlx + sqlc)
- Cache/distributed guard: Redis
- Auth: JWT (HMAC)

## Fitur Utama

- `POST /api/v1/auth/login` untuk mendapatkan access token.
- `GET /api/v1/inquiries/balance` untuk cek saldo user.
- `POST /api/v1/withdrawals` untuk tarik saldo.
- Idempotency untuk endpoint withdrawal (`X-Idempotency-Key`).
- Rate limiter berbasis Redis untuk withdrawal (default: 20 request/menit per user).
- Audit trail transaksi melalui tabel `wallet_ledger`.

## Arsitektur Singkat

Project menggunakan layering:

1. Handler (HTTP transport)
2. Service (business logic)
3. Repository (akses database)
4. Shared components (jwt, hash, idempotency, ratelimit, config, sqlc)

Alur utama withdrawal:

`HTTP Handler -> Service -> Repository -> SQL atomic update + ledger insert (transaction)`

## Struktur Project

```text
.
├── main.go
├── internal
│   ├── app            # bootstrap, module wiring, routes, lifecycle
│   ├── handlers       # layer HTTP
│   ├── services       # layer use-case / business logic
│   ├── repository     # layer persistence
│   ├── domain         # entity + value objects + domain errors
│   ├── middlewares    # jwt, idempotency, rate limiter, logging, recovery
│   └── shared         # config, jwt, hash, idempotency, ratelimit, sqlc, uid, log
├── db
│   ├── migrations     # file migration
│   ├── seeds          # data seed
│   └── queries        # query SQL untuk sqlc
├── docker-compose-infra.yaml
└── Makefile
```

## Prasyarat

- Go 1.25+
- Docker + Docker Compose

## Setup Cepat

1. Copy konfigurasi lokal:

```bash
make setup
```

2. Naikkan infrastruktur lokal (Postgres auth, Postgres wallet, Redis):

```bash
make infra-up
```

Data seed default:

- Email: `sample.user@example.com`
- Password: `password`

Seeder dijalankan lewat target `Makefile` dan dieksekusi oleh Goose.

## Menjalankan Single Instance

Mode ini menjalankan semua modul dalam satu proses (`auth + inquiry + withdraw`) pada port default `8080`.

1. Inisialisasi DB single (sekali saja):

```bash
make setup-single
```

2. Jalankan service:

```bash
go run .
```

## Menjalankan Multi Instance

Mode ini memisahkan proses:

- Inquiry/Auth instance (port default `8081`)
- Withdraw instance (port default `8082`)

1. Inisialisasi migration dan seed:

```bash
make setup-multi
```

2. Jalankan inquiry/auth instance:

```bash
go run . --bin=inquiry
```

3. Jalankan withdraw instance (terminal lain):

```bash
go run . --bin=withdraw
```

Catatan penting multi instance:

- `security.jwt.secret` pada `config.inquiry.yaml` dan `config.withdraw.yaml` harus sama.
- Login dilakukan ke inquiry instance, withdrawal ke withdraw instance.

## Contoh Workflow API

### 1) Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"sample.user@example.com","password":"password"}'
```

Simpan `access_token` dari response.

### 2) Balance Inquiry

```bash
curl http://localhost:8080/api/v1/inquiries/balance \
  -H 'Authorization: Bearer <access_token>'
```

### 3) Withdrawal

```bash
curl -X POST http://localhost:8080/api/v1/withdrawals \
  -H 'Authorization: Bearer <access_token>' \
  -H 'Content-Type: application/json' \
  -H 'X-Idempotency-Key: wd-001' \
  -d '{"amount_minor":100000}'
```

Untuk multi instance, ganti host/port sesuai service:

- login + inquiry: `http://localhost:8081`
- withdrawal: `http://localhost:8082`

## Menjalankan Test

```bash
go test ./...
```

## Endpoint Ringkas

- `GET /healthz`
- `POST /api/v1/auth/login`
- `GET /api/v1/inquiries/balance` (JWT)
- `POST /api/v1/withdrawals` (JWT + `X-Idempotency-Key`)

## Shutdown Infra

```bash
make infra-down
```
