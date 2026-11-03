# Rencana Remediasi Keamanan — finance-service

Tanggal audit: 2026-10-25. Sumber: security-findings.json. Urutan dalam tiap prioritas = urutan pengerjaan yang disarankan.

## P0 — Segera (hari ini – 3 hari): eksposur langsung / dapat dieksploitasi dengan prasyarat minimal.

| ID | Judul | Severity | Endpoint/area | Tindakan utama | Verifikasi |
|---|---|---|---|---|---|
| FIN-01 | JWT signing secret & kredensial lain pernah di-commit ke git dan masih dipakai | Critical | Semua endpoint ber-Auth(JWT) (83 endpoint) | 1) Rotasi JWT.SECRET, SECRET.KEY, SMTP password, dan DB password lama SEKARANG (anggap sudah bocor) — koordinasikan dengan service penerbit token karena secret dipakai bersama. | CI job gitleaks wajib hijau; test startup gagal bila JWT secret < 32 byte atau sama dengan nilai yang diketahui bocor (cek hash). |
| FIN-02 | Endpoint laporan realisasi anggaran tanpa autentikasi | High | GET /laporan/realisasi-anggaran, GET /laporan/export | Tambahkan middleware.Auth(tokenService) ke grup /laporan; bila laporan hanya untuk role tertentu, tambahkan RequirePermission. | Test iterasi app.GetRoutes(): untuk setiap route non-OPTIONS, request tanpa Authorization harus 401. |
| FIN-03 | SQL injection pada parameter sortColumn/sortOrder (ORDER BY) | High | GET /vendor, GET /directorate, GET /sub-direktorat | Gunakan allow-list kolom dan normalisasi arah (ASC/DESC) seperti yang sudah dilakukan di chart_of_account_repository.go & kegiatan_repository.go; atau clause.OrderByColumn dengan kolom yang divalidasi.. | Unit test repository/controller: sortColumn='vendor_name;--', '(select 1)', 'created_at desc, (case ...)' harus jatuh ke default; sortOrder selain asc/desc diabaikan. |

## P1 — 1–2 minggu: kontrol akses & integritas alur keuangan.

| ID | Judul | Severity | Endpoint/area | Tindakan utama | Verifikasi |
|---|---|---|---|---|---|
| FIN-04 | Tidak ada otorisasi berbasis role (function-level) selain /settings | High | POST /kegiatan/status/:id, PUT /apb/:id/status, PUT /pengeluaran/:id/status, PUT /apb/:id … | Definisikan matriks role x aksi (maker, approver anggaran, approver pembayaran, admin) dan terapkan middleware.RequirePermission per route/aksi (map menuRoles yang sudah ada dapat diperluas). | Tabel test per route: token role X -> status 403/2xx sesuai matriks. |
| FIN-05 | Pembatasan data per subdirektorat ditentukan oleh parameter klien (BOLA/IDOR) | High | GET /kegiatan, GET /kegiatan/summary, GET /kegiatan/:id, GET /apb … | Ambil scope (subdirektorat/UPT user) dari sumber tepercaya di server (claim JWT atau lookup finance.user_subdirektorat berdasarkan user_id token), bukan dari query string. | Test: user unit A + subdirektorat_id kosong -> hanya data unit A; user unit A akses GET /kegiatan/:id milik unit B -> 404/403. |
| FIN-06 | Persetujuan kegiatan (RAPB) tanpa validasi transisi status dan tanpa row lock | High | POST /kegiatan/status/:id | Implementasikan state machine seperti di apbRepository.StatusResponse: SELECT ... | Test: Reject->Approved ditolak; Approved->Approved ditolak; self-approve ditolak; 2 goroutine approve bersamaan -> 1 budget_lock. |
| FIN-07 | Mass assignment pada update APB: nominal pencairan dan URL dokumen dapat diisi bebas | High | PUT /apb/:id | Pisahkan DTO input dari kolom tersimpan: jangan bind dokumen_memo/bukti_transfer dari body (tag form:"-"); URL hanya dari processFileUpload. | Test: PUT tanpa file tapi dengan field bukti_transfer -> status tidak berubah, URL tidak tersimpan; jumlah_dicairkan negatif -> 400; ubah nominal setelah Sudah Dibayarkan -> 409. |
| FIN-14 | Secret di-bake ke image container dan token GitLab sebagai build-arg | Medium | (deployment) | Injeksi konfigurasi saat runtime (env var/secret Kubernetes), jangan COPY .env ke image. | CI: `docker history`/`dive` pada image tidak memuat .env; scan image (trivy --scanners secret). |

## P2 — 2–6 minggu: hardening alur bisnis, ketersediaan, dan observabilitas.

| ID | Judul | Severity | Endpoint/area | Tindakan utama | Verifikasi |
|---|---|---|---|---|---|
| FIN-08 | Status penerimaan ditentukan klien (dapat langsung 'settled') | Medium | POST /penerimaan/create, PUT /penerimaan/update/:id | Allow-list nilai status; status 'settled' hanya lewat aksi verifikasi terpisah oleh role berwenang; create selalu 'draft'.. | Test: create dengan status=settled -> tersimpan draft atau 400; status tidak dikenal -> 400. |
| FIN-09 | Hard delete data keuangan tanpa cek status dan tanpa audit trail | Medium | DELETE /kegiatan/:id, DELETE /penerimaan/delete/:id, DELETE /vendor/:id, DELETE /directorate/:id … | Larang delete untuk status di luar draft/revisi; gunakan soft delete (deleted_at, deleted_by) untuk data keuangan; catat audit log (user, waktu, objek, nilai sebelum).. | Test: DELETE kegiatan Approved -> 409; DELETE penerimaan -> baris tetap ada dengan deleted_at & deleted_by. |
| FIN-10 | Pesan error internal/database dikembalikan ke klien | Medium | GET /laporan/realisasi-anggaran (anonim), GET /laporan/export (anonim), GET /coa, GET /kegiatan/:id … | Log error lengkap di server (dengan trace_id), kembalikan pesan generik + trace_id ke klien. | Test: paksa repository mengembalikan error SQL -> body respons tidak mengandung 'SQLSTATE' atau nama tabel. |
| FIN-11 | Pencabutan token (blacklist) fail-open saat Redis tidak tersedia | Medium | Semua endpoint ber-Auth(JWT) (83 endpoint) | Di environment non-test, gagal startup bila Redis tidak bisa diinisialisasi; saat error Redis per request, tolak (fail-closed) atau batasi dengan TTL token pendek. | Test dengan mock Redis error -> Auth mengembalikan 401/503. |
| FIN-13 | Potensi resource exhaustion: limit tanpa batas atas, decode gambar tanpa batas dimensi, tanpa rate limit | Medium | GET list endpoints (vendor, directorate, sub-direktorat, apb, pengeluaran, penerimaan, kode-anggaran), POST /apb/create, PUT /apb/:id, PUT /pengeluaran/:id … | Batasi limit (mis. | Test: limit=100000 -> dibatasi; upload PNG 20000x20000 -> 400 tanpa alokasi besar. |

## P3 — Backlog terjadwal: hardening & kebersihan konfigurasi.

| ID | Judul | Severity | Endpoint/area | Tindakan utama | Verifikasi |
|---|---|---|---|---|---|
| FIN-12 | Validasi JWT kurang ketat (algoritma, iss/aud, exp wajib) dan parsing ganda v4/v5 | Low | Semua endpoint ber-Auth(JWT) (83 endpoint) | Gunakan satu library (jwt/v5) dan parser dengan jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuer(...), jwt.WithAudience(...). | Test: token tanpa exp -> 401; alg HS512/none -> 401; iss/aud salah -> 401. |
| FIN-15 | Koneksi database dengan sslmode=disable hardcoded | Low | (infrastruktur) | Gunakan nilai cfg.SSL (require/verify-full) dari konfigurasi, default verify-full di non-lokal.. | Test unit buildDsn: sslmode mengikuti konfigurasi. |
| FIN-16 | CORS wildcard dan tidak ada security headers | Low | (global) | Batasi AllowOrigins ke domain frontend resmi; tambahkan middleware helmet dan Cache-Control: no-store untuk respons API.. | Test: preflight dari origin tak dikenal tidak mendapat Access-Control-Allow-Origin. |
| FIN-17 | Ekstensi file upload (PDF) diambil dari nama file klien | Low | POST /apb/create, PUT /apb/:id, PUT /pengeluaran/:id, POST /penerimaan/create … | Tentukan ekstensi dari detectedType (application/pdf -> .pdf), abaikan nama file klien.. | Test: upload PDF bernama x.html -> objek tersimpan .pdf. |
| FIN-18 | Daftar pengguna (id, nama, role level) dapat diakses semua user login | Low | GET /dropdown/users | Batasi ke role admin/pengelola sub-direktorat (RequirePermission).. | Test: role non-admin -> 403. |
| FIN-19 | Default Locals role_level_id=1 untuk semua request & route terdaftar dua kali | Informational | (global) | Hapus middleware default role_level_id; panggil RegisterRoutes hanya sekali.. | Test: route count == jumlah route unik. |

## Rencana retest

1. Setelah P0 selesai: retest FIN-01..03 (token palsu dengan secret lama harus ditolak; /laporan tanpa token 401; payload ORDER BY tidak berpengaruh).
2. Setelah P1: jalankan test matriks role x endpoint dan test scope subdirektorat (FIN-04, FIN-05), test state machine kegiatan & APB (FIN-06, FIN-07).
3. Jalankan govulncheck & gosec (setelah disetujui) dan staticcheck versi Go 1.25; tambahkan ke pipeline CI.
4. Dynamic testing terbatas di staging dengan akun dummy per role dan data sintetis.
