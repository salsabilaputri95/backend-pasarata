# Pasara'ta Backend API

Backend API untuk aplikasi Pasara'ta berbasis Go dengan PostgreSQL.

## Prasyarat

- Go 1.22+
- PostgreSQL 15+
- Git

## Instalasi

1. Salin file environment contoh:
   ```bash
   copy .env.example .env
   ```
2. Sesuaikan konfigurasi database dan JWT:
   ```env
   PORT=8080
   JWT_SECRET=change-this-secret
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=postgres
   DB_NAME=pasarata
   DB_SSLMODE=disable
   FRONTEND_URL=http://localhost:3000
   ```
3. Buat database PostgreSQL:
   ```sql
   CREATE DATABASE pasarata;
   ```
4. Jalankan dependency:
   ```bash
   go mod download
   ```

## Menjalankan aplikasi

```bash
go run ./cmd/api
```

Aplikasi akan berjalan di `http://localhost:8080`.

## Default akun

- Username: `admin`
- Password: `admin123`

## Endpoint utama

- `POST /api/login`
- `GET /api/me`
- `GET /api/dashboard`
- `GET /api/entries/me`
- `POST /api/entries`
- `GET /api/admin/dashboard` (Admin only)

## Struktur proyek

- `cmd/api` — entry point aplikasi
- `internal/config` — konfigurasi environment
- `internal/db` — koneksi database dan migrasi
- `internal/handlers` — handler HTTP
- `internal/middleware` — autentikasi & authorization
- `internal/models` — model data
- `internal/routes` — routing API
- `internal/services` — validasi & bisnis logic
- `migrations` — SQL schema
