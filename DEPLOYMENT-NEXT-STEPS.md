# Backend Deployment: Langkah Selanjutnya

Dokumen ini berisi urutan untuk menyiapkan backend, menguji Docker, lalu menghubungkannya ke Render, aplikasi mobile, dan dashboard web.

## 1. Siapkan konfigurasi lokal

Jalankan dari folder `BE_web`:

```powershell
if (!(Test-Path .env)) { Copy-Item .env.example .env }
```

Edit `.env` dan isi nilai lokal:

- `DB_HOST=localhost` untuk menjalankan API langsung dengan `go run`.
- `DB_PORT=5432` untuk PostgreSQL lokal yang sudah terpasang.
- `DB_USER`, `DB_PASSWORD`, dan `DB_NAME` sesuai database lokal.
- `JWT_SECRET` dengan nilai acak minimal 32 karakter.
- `CLOUDINARY_CLOUD_NAME`, `CLOUDINARY_API_KEY`, dan `CLOUDINARY_API_SECRET` dari pengaturan Cloudinary. Folder upload memakai `crud_image`.

Jangan commit `.env` atau membagikan API secret. Secret Cloudinary pernah berada di template konfigurasi dan dibagikan di chat, jadi rotasi sebelum deployment; jika repository sempat dipush, anggap secret lama sudah terekspos. Jangan gunakan password atau JWT contoh untuk production.

## 2. Jalankan backend tanpa Docker

Pastikan PostgreSQL hidup dan kedua migration sudah diterapkan ke database target, kemudian:

```powershell
go run .\cmd\server
```

Periksa status API dan koneksi database:

```powershell
Invoke-RestMethod http://localhost:8080/healthz
```

## 3. Jalankan dengan Docker Compose

Docker Desktop harus berjalan. Compose membaca `.env` yang sama, membangun image backend, dan menjalankan PostgreSQL dengan volume persisten:

```powershell
docker compose config --quiet
docker compose up --build
```

Di dalam Compose, backend mengakses database melalui hostname `db`. Dari Windows, API tersedia di port `8080`, sedangkan PostgreSQL tersedia di `localhost:5434`.

Periksa container dan health API dari terminal lain:

```powershell
docker compose ps
Invoke-RestMethod http://localhost:8080/healthz
```

Hentikan layanan tanpa menghapus database:

```powershell
docker compose down
```

Hanya gunakan `docker compose down -v` jika memang ingin menghapus volume dan seluruh data PostgreSQL lokal. File migration di `/docker-entrypoint-initdb.d` dijalankan otomatis hanya ketika volume database masih kosong; perubahan migration berikutnya harus diterapkan secara sengaja ke database yang sudah ada.

## 4. Siapkan database production

Gunakan PostgreSQL terkelola, misalnya database Render atau Supabase. Catat host, port, user, password, nama database, dan mode SSL dari provider. Jalankan migration secara berurutan pada database tersebut:

1. `migrations/001_erd_database.sql`
2. `migrations/002_inventory_workflow.sql`

Jangan menjalankan migration production menggunakan volume database lokal Docker. Backup database production sebelum perubahan skema.

## 5. Deploy backend ke Render

1. Pastikan repository dan Blueprint `render.yaml` tersedia pada branch yang akan dideploy. Jika Blueprint berada di subfolder, pilih lokasi Blueprint tersebut saat membuat Blueprint di Render.
2. Buat Blueprint dari repository. Service menggunakan Dockerfile di folder backend dan health check `/healthz`.
3. Isi environment yang ditandai `sync: false`: `CORS_ORIGIN`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, serta kredensial Cloudinary. `JWT_SECRET` dibuat oleh Blueprint; jangan menimpanya dengan nilai contoh.
4. Gunakan `DB_SSLMODE=require` untuk database managed yang mensyaratkan SSL.
5. Pastikan migration production sudah diterapkan, lalu tunggu deploy selesai dan health check berhasil.
6. Catat URL HTTPS service Render. Gunakan URL ini sebagai base URL API untuk mobile dan web.

Render tidak membaca `.env` lokal. Semua variabel production harus diatur melalui environment service di dashboard Render atau Blueprint secrets.

## 6. Sambungkan dan uji aplikasi

- Atur aplikasi mobile agar mengirim permintaan SKU ke URL Render menggunakan bearer token dan foto dalam format data URI JPEG, PNG, atau WebP. Foto diunggah oleh backend ke Cloudinary.
- Atur dashboard web menggunakan URL API yang sama. Dashboard membaca SKU dan URL foto dari endpoint GET; upload foto dilakukan oleh aplikasi mobile.
- Set `CORS_ORIGIN` pada Render ke origin dashboard web, termasuk protokol `https://`, tanpa path.
- Uji login, GET daftar SKU, submit SKU beserta foto, tampilan foto di web, dan alur persetujuan owner.
- Pastikan API secret Cloudinary hanya tersimpan di environment backend, tidak di aplikasi mobile atau bundle web.

## 7. Jika ada masalah

- Docker menolak `DB_PASSWORD`: pastikan `.env` dibuat dari template dan password lokal sudah diisi.
- `docker compose config` gagal: jalankan perintah dari folder `BE_web` yang memiliki `docker-compose.yml` dan `.env`.
- Health check gagal: lihat `docker compose logs api db`, lalu periksa host/port database dan status kedua migration.
- API di Render gagal start: periksa seluruh environment wajib, khususnya `JWT_SECRET`, koneksi database, SSL mode, dan tiga credential Cloudinary.
- Foto gagal diunggah: pastikan Cloudinary cloud name/key/secret benar dan request mengirim data URI dengan tipe MIME yang didukung.
