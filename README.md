# Dracin Reels Backend

Backend API drama pendek vertikal ala Melolo — **API SAJA, tanpa halaman
HTML**. Kliennya ada dua, dan keduanya memakai API yang sama:

- **Web admin** (desktop, lengkap — repo terpisah `dracin-reels-web-admin`)
- **Aplikasi Flutter** (mode penonton + mode Admin — repo terpisah)

Stack: **Go + chi** (router standar, ringan) + **PostgreSQL 13**,
arsitektur berlapis: `cmd/server` → `internal/presentation/http`
→ `internal/application` (usecase) → `internal/infrastructure`
(repository PostgreSQL) → `internal/domain` (entitas + aturan).

Model bisnis lengkap: lihat [`DOMAIN_MODEL.md`](DOMAIN_MODEL.md).
Skema: `internal/infrastructure/database/migrations/001_schema.sql`.
Seed: `internal/infrastructure/database/seed/001_seed.sql`.

## Yang JUJUR belum ada di fase ini

- ❌ Pembayaran beneran (paket koin & VIP hanya katalog/klaim sampel;
  `POST /vip/claim` tidak menagih apa pun).
- ❌ Iklan beneran (tugas `watch_ad` hanya katalog; klaimnya belum dibuka).
- ❌ Streaming/CDN video beneran — `video_url`/`subtitle_url` seed adalah
  URL placeholder (`example.com`/`placehold.co`).
- ❌ HTTPS (proxy nginx saat ini HTTP di port 8095).
- ⚠️ Media upload tahap ini = direktori media VPS, diserve backend hanya
  sebagai **staging/preview admin**. Untuk tayang publik skala besar,
  media **HARUS pindah ke object storage + CDN** (aturan lama reels:
  video jangan diserve dari VPS kecil untuk publik massal). URL
  `/media/...` bukan distribusi publik massal, dan file media tidak
  diproteksi auth maupun kunci episode di level file.

## Konfigurasi

Salin `.env.example` menjadi `.env`, isi nilainya. **Jangan pernah
commit `.env` dan jangan hardcode kredensial di repo.**

| Variabel | Keterangan |
|---|---|
| `PORT` | port internal (deploy VPS: `8100`) |
| `DB_HOST/DB_PORT/DB_NAME/DB_USER/DB_PASSWORD` | koneksi PostgreSQL, atau `DATABASE_URL` |
| `JWT_SECRET` | rahasia JWT (wajib diganti di produksi) |
| `MEDIA_DIR` | direktori upload (deploy VPS: `/opt/dracin-reels-backend/media`) |
| `SEED_DEMO` | `true` = seed demo bila tabel users masih kosong |

Jalankan:

```bash
go build -o dracin-reels-backend ./cmd/server
./dracin-reels-backend   # migrasi + seed otomatis bila users kosong
```

## Kontrak login (web & mobile SAMA)

```
POST /api/v1/auth/login
{ "email": "admin@demo.dracin.local", "password": "Admin123!" }
-> { "success": true, "data": { "token": "<JWT>", "user": { "id": 1, "role": "admin", ... } } }
```

- `user` selalu memuat `role` (`viewer`/`admin`); JWT claims juga memuat
  `role` yang sama, sehingga guard admin bisa diuji dari token saja.
- Tanpa token → **401**. Token viewer mengakses endpoint admin → **403**.
  Token tidak valid → 401. Error selalu JSON jelas, **tidak pernah
  sukses palsu** — khususnya untuk upload: bila backend belum terhubung,
  klien wajib menampilkan error, bukan berpura-pura upload berhasil.

## Endpoint REST `/api/v1`

Semua respons JSON konsisten: sukses `{success:true, data, meta?}`,
gagal `{success:false, error:{code, message}}` (pesan Bahasa Indonesia).
Pagination: `?page=&limit=` (default 1/20, maks 100).

### Publik
- `GET /health`, `GET /up` — status (di luar `/api/v1`)
- `GET /series?genre=<slug>&page=&limit=` — hanya `published`
- `GET /series/{id}` — series + episode (flag `locked`, `video_url` kosong bila terkunci)
- `GET /series/{id}/episodes`, `GET /episodes/{id}`, `GET /genres`
- `GET /coin-packages` (hanya aktif), `GET /rewards/tasks`
- `GET /episodes/{id}/comments`

### Butuh login (JWT)
- `POST /auth/register`, `GET /auth/me`
- `GET /wallet`, `GET /wallet/transactions`
- `POST /unlock {episode_id}` — cek VIP/koin, potong saldo atomik
- `POST /checkin` — sekali/hari, klaim kedua = error jelas
- `GET /vip`, `POST /vip/claim {plan: weekly|monthly|yearly}` (sampel, tanpa pembayaran)
- `PUT /watch-progress {episode_id, position_seconds, finished}` (episode terkunci ditolak)
- `GET /continue-watching`, `GET/POST/DELETE /bookmarks/{series_id}`,
  `POST/DELETE /likes/{episode_id}`, `POST /episodes/{id}/comments`

### Admin (JWT + role=admin; viewer = 403, tanpa token = 401)
- CRUD: `GET/POST/PUT/DELETE /admin/series[/{id}]`,
  `POST/PUT/DELETE /admin/episodes[/{id}]`,
  `POST/PUT/DELETE /admin/genres[/{id}]`,
  `GET/POST/PUT/DELETE /admin/coin-packages[/{id}]`
- Video episode punya DUA sumber (klien wajib sediakan keduanya, salah
  satu cukup): (1) upload file di bawah ini, atau (2) `video_url`
  eksternal http(s) yang ditempel admin lewat CRUD episode
  (mis. link object storage/CDN sendiri) — tanpa file upload.
  Validasi: `video_url` harus http(s) valid (atau kosong, atau path
  `media/...` hasil upload); skema lain ditolak 400. Bila keduanya
  diisi, upload yang menang dan menimpa `video_url`.
- Upload (multipart, field `file`):
  - `POST /admin/series/{id}/cover` — gambar png/jpg/jpeg/webp, maks 5 MB → `cover_url`
  - `POST /admin/episodes/{id}/video` — video mp4/mov/mkv, maks 500 MB → `video_url`
  - `POST /admin/episodes/{id}/subtitle` — subtitle vtt/srt, maks 5 MB → `subtitle_url`
  - `POST /admin/episodes/{id}/thumbnail` — gambar png/jpg/jpeg/webp, maks 5 MB → `thumbnail_url`
  - Nilai tersimpan = path relatif base API, mis. `media/episodes/....mp4`;
    diakses sebagai `<api-base>/media/episodes/....mp4`.
    Nama file diamankan (tanpa path traversal) dan dibuat unik.
- Media staging: `GET /media/...` (disserve backend, lihat peringatan CDN di atas)

## Aturan bisnis inti

Episode `1..free_episode_count` gratis; sisanya terkunci sampai dibuka
dengan koin (`coin_price`) atau VIP aktif. VIP berakhir = kembali
terkunci (kecuali sudah di-unlock koin). Unlock memotong saldo dalam
satu transaksi; saldo kurang = error jelas. Check-in harian sekali/hari.

## Seed demo

| Akun | Email | Password |
|---|---|---|
| Admin demo | `admin@demo.dracin.local` | `Admin123!` |
| Viewer demo | `viewer@demo.dracin.local` | `Viewer123!` |

⚠️ **Akun demo ini HARUS diganti/dihapus sebelum data asli masuk.**
Viewer demo bersaldo 200 koin. Series seed: **CINTA SANG PENGUASA QIN**
(karya orisinal, bukan konten Melolo bajakan) — 5 episode, 2 gratis,
episode 3/4/5 seharga 50/80/100 koin; genre Romansa, Balas Dendam,
Fantasi, Misteri; 4 paket koin; 3 tugas hadiah (check-in 10 koin/hari).

## Deploy VPS (ringkas)

Lokasi: `/opt/dracin-reels-backend`, service systemd
`dracin-reels-backend`, port internal `8100`, proxy nginx port 8095
di lokasi `/dracin-api/` → `http://127.0.0.1:8100/`.
URL publik: `http://103.147.33.11:8095/dracin-api/`
(mis. `.../dracin-api/health`, `.../dracin-api/api/v1/series`).
File deploy: `deploy/` (service systemd + snippet nginx).
