# Pasara'ta Backend API

Backend API untuk **Pasara'ta** — Sistem Digitalisasi Pendataan Harga Komoditas Pasar BPS Jeneponto.

Service backend ini dibangun menggunakan bahasa pemrograman **Go (Golang)** dengan web framework **Gin**, ORM **GORM**, dan basis data **PostgreSQL**. Backend menyediakan layanan RESTful API berkinerja tinggi, aman dengan autentikasi JWT, serta mendukung kalkulasi otomatis, validasi anomali harga, import/export spreadsheet, dan audit trail histori perubahan data.

---

## 📌 Deskripsi Singkat

Pasara'ta mendigitalisasi proses survei dan pendataan harga komoditas pasar tradisional di wilayah Kabupaten Jeneponto. Backend ini mengotomatisasi:
- Perhitungan konversi harga satuan lokal ke satuan standar BPS.
- Deteksi anomali harga (di bawah batas minimum atau di atas batas maksimum) secara *non-blocking* (tidak memblokir input petugas).
- Pengambilan referensi harga historis/tahun sebelumnya secara otomatis.
- Pencatatan seluruh jejak aktivitas pengguna (*audit trail*) baik saat penambahan, pembaruan, pembatalan, maupun penghapusan data.
- Pengolahan data analitik berupa rekapitulasi min/max/avg, komparasi harga antar periode/tahun, serta import dan export dokumen Excel/CSV.

---

## 🚀 Fitur Utama

### 1. 🔐 Autentikasi & Otorisasi
- **JWT (JSON Web Token)** untuk otentikasi stateless yang aman.
- **Role-Based Access Control (RBAC)** dengan pemisahan hak akses:
  - `admin`: Mengelola seluruh master data, akun petugas, penugasan, monitoring, import/export, audit, dan seluruh data entri.
  - `collector`: Mengakses data referensi pasar yang ditugaskan, input data survei, melihat, memperbarui, dan membatalkan data milik sendiri.
- Endpoint profil pengguna terautentikasi (`GET /api/me`).

### 2. 📝 Modul Petugas Pendata (Collector)
- **Input Data Survei (`POST /api/entries`)**:
  - Validasi penugasan (petugas hanya dapat menginput pasar yang ditugaskan).
  - Kalkulasi konversi otomatis berdasarkan bobot dan faktor konversi satuan standar.
  - Penentuan status harga otomatis (`normal`, `below_minimum`, `above_maximum`).
- **Referensi Harga Historis (`GET /api/price-reference`)**: Mengambil batas referensi min, max, dan harga periode sebelumnya per komoditas & pasar.
- **Kelola Data Mandiri**:
  - Melihat daftar entri yang telah diinput (`GET /api/entries/me`) dengan filter tahun.
  - Memperbarui entri data sendiri (`PUT /api/entries/:id`).
  - Membatalkan/soft-deactivate entri data sendiri (`PATCH /api/entries/:id/deactivate`).
  - Melihat riwayat audit entri terkait (`GET /api/entries/:id/audit`).
- **Dashboard Petugas (`GET /api/dashboard`)**: Menampilkan statistik pasar penugasan aktif, total entri yang telah diinput, jumlah data warning, dan entri yang dapat diedit.

### 3. 🏢 Modul Administrator
- **Dashboard Monitoring & Sebaran Data (`GET /api/admin/dashboard`)**:
  - Kartu metrik total pendata, pasar, komoditas, total data entri, dan data warning.
  - Distribusi data breakdown per tahun, per pasar, per pendata, dan 10 entri terbaru.
- **Manajemen Petugas Pendata (`/api/admin/collectors`)**:
  - Tambah akun pendata baru, ubah data pendata, aktif/nonaktifkan akun, dan reset password.
- **Manajemen Penugasan Pasar (`/api/admin/assignments`)**:
  - Penugasan relasi many-to-many antara petugas pendata dan pasar.
  - Tambah penugasan baru dan hapus penugasan.
- **Manajemen Master Data**:
  - **Pasar (`/api/admin/markets`)**: CRUD data pasar, provinsi, kabupaten/kota, kecamatan, dan NKS.
  - **Kategori Komoditas (`/api/admin/categories`)**: CRUD kategori komoditas.
  - **Komoditas (`/api/admin/commodities`)**: CRUD komoditas dan pengelompokan kategori.
  - **Satuan & Standar (`/api/admin/units`)**: CRUD satuan lokal, penetapan satuan standar, dan faktor konversi.
- **Review & Verifikasi Data Entri (`/api/admin/entries`)**:
  - Filter pencarian entri menyeluruh (tahun, pasar, pendata, status warning, kata kunci).
  - Update data entri oleh admin (`PUT /api/admin/entries/:id`).
  - Hapus data entri (`DELETE /api/admin/entries/:id`).
- **Analisis & Rekapitulasi Harga**:
  - **Rekap Harga (`GET /api/admin/summary`)**: Agregasi harga minimum, maksimum, dan rata-rata per pasar dan komoditas.
  - **Komparasi Harga (`GET /api/admin/comparison`)**: Analisis fluktuasi harga tahun berjalan vs tahun sebelumnya beserta selisih dan persentase perubahan.
- **Import Data Cerdas (`/api/admin/import/*`)**:
  - Inspeksi nama header kolom Excel/CSV secara fleksibel.
  - Validasi dan pratinjau (*preview*) data entri maupun data master sebelum disimpan ke database (*commit*).
- **Export Laporan Multi-Format (`/api/admin/export*`)**:
  - Export data ke format Excel (**XLSX**) dan **CSV**.
  - Pilihan cakupan export: data entri detail, ringkasan summary, maupun tabel perbandingan harga.
- **Audit Trail & Logging (`GET /api/admin/audit-logs`)**:
  - Pencatatan seluruh aksi pengguna (`create`, `update`, `delete`, `deactivate`, `import`) lengkap dengan snapshot data *before* & *after*, user pelaksana, dan timestamp.

---

## 🛠️ Teknologi & Dependensi

| Kategori | Teknologi | Deskripsi |
|---|---|---|
| **Bahasa** | Go 1.22+ | Bahasa pemrograman utama berkinerja tinggi |
| **HTTP Framework** | Gin Web Framework (`gin-gonic/gin`) | Routing dan handling HTTP cepat & minimalis |
| **ORM / Database Driver** | GORM (`gorm.io/gorm`, `driver/postgres`) | Pemetaan objek relasional & query PostgreSQL |
| **Database** | PostgreSQL 15+ | Penyimpanan data relasional |
| **Keamanan & Auth** | Golang-JWT v5 (`golang-jwt/jwt/v5`) & Bcrypt (`golang.org/x/crypto`) | Enkripsi kata sandi dan manajemen token JWT |
| **Spreadsheet Engine** | Excelize v2 (`xuri/excelize/v2`) | Pembacaan & pembuatan file Excel (.xlsx) canggih |
| **Environment** | GoDotEnv (`joho/godotenv`) | Pembacaan konfigurasi file `.env` |
| **CORS** | Gin Contrib CORS (`gin-contrib/cors`) | Manajemen kebijakan Cross-Origin Resource Sharing |

---

## 📋 Prasyarat Sistem

Pastikan telah menginstal perangkat lunak berikut:
- **Go**: versi 1.22 atau lebih baru ([Unduh Go](https://golang.org/dl/))
- **PostgreSQL**: versi 15 atau lebih baru
- **Git**

---

## ⚙️ Instalasi & Konfigurasi

1. **Clone repository atau buka direktori backend:**
   ```bash
   cd backend-pasarata
   ```

2. **Salin file konfigurasi environment:**
   ```bash
   copy .env.example .env
   ```

3. **Sesuaikan variabel environment di dalam file `.env`:**
   ```env
   PORT=
   JWT_SECRET=
   DB_HOST=
   DB_PORT=
   DB_USER=
   DB_PASSWORD=
   DB_NAME=
   DB_SSLMODE=
   FRONTEND_URL=
   ```

4. **Buat database di PostgreSQL:**
   ```sql
   CREATE DATABASE pasarata;
   ```

5. **Unduh dependensi Go:**
   ```bash
   go mod download
   ```

---

## 🏃 Menjalankan Aplikasi

Jalankan server API dengan perintah:

```bash
go run ./cmd/api
```

Server API akan aktif dan mendengarkan request di: `http://localhost:8080`

Saat pertama kali dijalankan, sistem akan otomatis:
- Menjalankan migrasi skema tabel database.
- Melakukan *seeding* akun Administrator default.

---

## 👤 Akun Bawaan (Default Seed)

- **Username**: `admin`
- **Password**: `admin123`
- **Role**: `admin`

---

## 📡 Daftar Endpoint API

### 🔑 Autentikasi & Profil
| Metode | Endpoint | Hak Akses | Deskripsi |
|---|---|---|---|
| `POST` | `/api/login` | Publik | Autentikasi pengguna & mendapatkan JWT token |
| `GET` | `/api/me` | Terautentikasi | Mendapatkan detail profil pengguna yang sedang login |

### 📋 Pendataan & Referensi Petugas
| Metode | Endpoint | Hak Akses | Deskripsi |
|---|---|---|---|
| `GET` | `/api/dashboard` | Terautentikasi | Ringkasan statistik dashboard petugas pendata |
| `GET` | `/api/entries/me` | Terautentikasi | Daftar entri harga yang diinput oleh petugas yang login |
| `POST` | `/api/entries` | Terautentikasi | Menambahkan entri pendataan harga baru |
| `PUT` | `/api/entries/:id` | Terautentikasi | Memperbarui entri harga milik sendiri |
| `PATCH` | `/api/entries/:id/deactivate` | Terautentikasi | Membatalkan / menonaktifkan entri harga sendiri |
| `GET` | `/api/entries/:id/audit` | Terautentikasi | Melihat riwayat perubahan/audit entri tertentu |
| `GET` | `/api/price-reference` | Terautentikasi | Mengambil referensi harga komoditas & pasar historis |
| `GET` | `/api/markets` | Terautentikasi | Mengambil daftar pasar aktif |
| `GET` | `/api/categories` | Terautentikasi | Mengambil daftar kategori komoditas aktif |
| `GET` | `/api/commodities` | Terautentikasi | Mengambil daftar komoditas aktif |
| `GET` | `/api/units` | Terautentikasi | Mengambil daftar satuan aktif |

### 🛠️ Administrasi (Role: Admin Only)
| Metode | Endpoint | Deskripsi |
|---|---|---|
| `GET` | `/api/admin/dashboard` | Statistik dashboard admin & breakdown sebaran data |
| `GET` / `POST` | `/api/admin/collectors` | List & tambah akun petugas pendata |
| `PUT` / `PATCH` | `/api/admin/collectors/:id` | Update data & ubah status aktif pendata |
| `POST` | `/api/admin/collectors/:id/reset-password` | Reset password akun petugas pendata |
| `GET` / `POST` | `/api/admin/markets` | List & tambah master data pasar |
| `PUT` / `PATCH` | `/api/admin/markets/:id` | Update data & ubah status pasar |
| `GET` / `POST` | `/api/admin/assignments` | List & buat penugasan pasar ke pendata |
| `DELETE` | `/api/admin/assignments/:id` | Menghapus penugasan pasar |
| `GET` / `POST` | `/api/admin/categories` | List & tambah master kategori komoditas |
| `PUT` / `PATCH` | `/api/admin/categories/:id` | Update & ubah status kategori |
| `GET` / `POST` | `/api/admin/commodities` | List & tambah master komoditas |
| `PUT` / `PATCH` | `/api/admin/commodities/:id` | Update & ubah status komoditas |
| `GET` / `POST` | `/api/admin/units` | List & tambah master satuan & faktor konversi |
| `PUT` / `PATCH` | `/api/admin/units/:id` | Update & ubah status satuan |
| `GET` | `/api/admin/entries` | Monitoring & review seluruh entri data dengan filter |
| `PUT` | `/api/admin/entries/:id` | Update entri data oleh admin |
| `DELETE` | `/api/admin/entries/:id` | Menghapus entri data harga |
| `GET` | `/api/admin/summary` | Rekapitulasi harga min/max/rata-rata per pasar |
| `GET` | `/api/admin/comparison` | Analisis perbandingan harga antar periode tahun |
| `POST` | `/api/admin/import/headers` | Inspeksi header kolom file Excel/CSV |
| `POST` | `/api/admin/import/preview` | Preview & validasi baris file import entri |
| `POST` | `/api/admin/import/commit` | Eksekusi simpan import data entri |
| `POST` | `/api/admin/import/master/preview` | Preview & validasi import master data |
| `POST` | `/api/admin/import/master/commit` | Eksekusi simpan import master data |
| `GET` | `/api/admin/export` | Export data entri ke format CSV |
| `GET` | `/api/admin/export-report` | Export laporan terpadu ke format XLSX / CSV |
| `GET` | `/api/admin/audit-logs` | Mengambil seluruh catatan riwayat audit sistem |

---

