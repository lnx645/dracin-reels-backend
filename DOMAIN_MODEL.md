# DOMAIN MODEL — Dracin Reels Backend

> Dokumen ini ditulis SEBELUM kode berat (Fase A). Bila ada yang kurang,
> dokumen ini dilengkapi — maknanya tidak diubah diam-diam.
> Bahasa teknis (nama tabel/kolom/endpoint) memakai Bahasa Inggris;
> pesan error API memakai Bahasa Indonesia yang jelas.

## 1. Gambaran

Dracin Reels adalah layanan drama pendek vertikal ala Melolo.
Backend ini melayani: katalog drama, pemutaran episode, dompet koin,
langganan VIP, progres tontonan, bookmark/suka/komentar, dan tugas hadiah.
Konten sampel adalah karya orisinal ("CINTA SANG PENGUASA QIN") — bukan
konten Melolo bajakan, bukan still/poster drama berhak cipta.

Yang BELUM ada di fase ini (dinyatakan jujur, bukan diklaim ada):
pembayaran beneran, iklan beneran, streaming/CDN video beneran, HTTPS.
`video_url`/`subtitle_url` sampel adalah URL placeholder.

## 2. Entitas

### User
| Field | Tipe | Keterangan |
|---|---|---|
| id | BIGINT PK | serial |
| name | TEXT | nama tampilan |
| email | TEXT UNIQUE | login, unik |
| password_hash | TEXT | bcrypt, tidak pernah dikembalikan API |
| avatar_url | TEXT NULL | |
| role | TEXT | `viewer` \| `admin` |
| created_at / updated_at | TIMESTAMPTZ | |

### Genre
| Field | Tipe | Keterangan |
|---|---|---|
| id | BIGINT PK | serial |
| name | TEXT | mis. Romansa |
| slug | TEXT UNIQUE | mis. `romansa` |

### Series / Drama
| Field | Tipe | Keterangan |
|---|---|---|
| id | BIGINT PK | serial |
| title | TEXT | judul drama |
| description | TEXT | sinopsis |
| cover_url | TEXT NULL | |
| status | TEXT | `draft` \| `published` |
| total_episode | INT | dihitung/diisi admin, konsisten dengan jumlah episode |
| free_episode_count | INT | jumlah episode awal yang gratis |
| rating | NUMERIC(3,1) | mis. 4.8 |
| created_at / updated_at | TIMESTAMPTZ | |

Relasi many-to-many ke Genre lewat tabel `series_genres(series_id, genre_id)`.

### Episode
| Field | Tipe | Keterangan |
|---|---|---|
| id | BIGINT PK | serial |
| series_id | BIGINT FK → series | |
| number | INT | nomor urut dalam series, unik per series |
| title | TEXT | |
| description | TEXT | |
| video_url | TEXT | placeholder di fase ini |
| subtitle_url | TEXT NULL | placeholder di fase ini |
| duration_seconds | INT | durasi detik |
| coin_price | INT | harga koin; `0` = gratis |
| status | TEXT | `draft` \| `published` |
| thumbnail_url | TEXT NULL | |
| created_at / updated_at | TIMESTAMPTZ | |

### CoinWallet
| Field | Tipe | Keterangan |
|---|---|---|
| user_id | BIGINT PK FK → users | satu dompet per user |
| balance | INT | saldo koin, tidak boleh negatif |
| updated_at | TIMESTAMPTZ | |

### CoinTransaction
| Field | Tipe | Keterangan |
|---|---|---|
| id | BIGINT PK | serial |
| user_id | BIGINT FK → users | |
| type | TEXT | `checkin` \| `task` \| `purchase` \| `unlock` \| `reward` |
| amount | INT | positif = masuk, negatif = keluar |
| balance_after | INT | saldo setelah transaksi |
| reference_id | TEXT NULL | mis. `episode:12`, `task:checkin:2026-10-30` |
| created_at | TIMESTAMPTZ | |

Pemetaan jenis dari spesifikasi ke `type` di atas:
`checkin`→`checkin`, `tugas`→`task`, `beli`→`purchase`,
`unlock`→`unlock`, `hadiah`→`reward`.

### CoinPackage
| Field | Tipe | Keterangan |
|---|---|---|
| id | BIGINT PK | serial |
| name | TEXT | mis. "Paket 550 Koin" |
| coin_amount | INT | koin pokok |
| price_idr | INT | harga rupiah (katalog saja di fase ini) |
| bonus_coins | INT | bonus, default 0 |
| active | BOOLEAN | hanya paket aktif tampil di katalog publik |
| created_at | TIMESTAMPTZ | |

### VipSubscription
| Field | Tipe | Keterangan |
|---|---|---|
| id | BIGINT PK | serial |
| user_id | BIGINT FK → users | |
| plan | TEXT | `weekly` \| `monthly` \| `yearly` |
| starts_at | TIMESTAMPTZ | |
| ends_at | TIMESTAMPTZ | |
| status | TEXT | `active` \| `expired` |
| auto_renew | BOOLEAN | dicatat saja; penagihan otomatis BELUM ada |
| created_at | TIMESTAMPTZ | |

Durasi paket sampel: weekly = 7 hari, monthly = 30 hari, yearly = 365 hari.

### WatchProgress
| Field | Tipe | Keterangan |
|---|---|---|
| user_id | BIGINT FK → users | bagian dari PK |
| episode_id | BIGINT FK → episodes | bagian dari PK |
| position_seconds | INT | posisi terakhir |
| finished | BOOLEAN | true bila selesai ditonton |
| last_watched_at | TIMESTAMPTZ | |

### Bookmark
| Field | Tipe | Keterangan |
|---|---|---|
| user_id | BIGINT FK → users | bagian dari PK |
| series_id | BIGINT FK → series | bagian dari PK |
| created_at | TIMESTAMPTZ | |

### Like
| Field | Tipe | Keterangan |
|---|---|---|
| user_id | BIGINT FK → users | bagian dari PK |
| episode_id | BIGINT FK → episodes | bagian dari PK |
| created_at | TIMESTAMPTZ | |

### Comment
| Field | Tipe | Keterangan |
|---|---|---|
| id | BIGINT PK | serial |
| user_id | BIGINT FK → users | |
| episode_id | BIGINT FK → episodes | |
| body | TEXT | isi komentar |
| created_at | TIMESTAMPTZ | |

### RewardTask
| Field | Tipe | Keterangan |
|---|---|---|
| id | BIGINT PK | serial |
| name | TEXT | mis. "Check-in Harian" |
| type | TEXT | `checkin` \| `watch_ad` \| `finish_series` |
| coin_reward | INT | hadiah koin |
| daily_limit | INT | batas klaim per hari (1 untuk check-in) |
| created_at | TIMESTAMPTZ | |

Pemetaan jenis dari spesifikasi: `checkin`→`checkin`,
`tonton_iklan`→`watch_ad`, `selesaikan_series`→`finish_series`.

### UserRewardClaim
| Field | Tipe | Keterangan |
|---|---|---|
| user_id | BIGINT FK → users | bagian dari UNIQUE(user_id, task_id, claim_date) |
| task_id | BIGINT FK → reward_tasks | |
| claim_date | DATE | tanggal klaim (WIB dihitung dari tanggal server) |
| created_at | TIMESTAMPTZ | |

### Tabel pendukung (bukan perubahan makna)
`episode_unlocks(user_id, episode_id, unlocked_at)` — mencatat episode
yang sudah dibuka dengan koin, agar status "terbuka" tidak hilang dan
tidak bergantung pada pemindaian riwayat transaksi. Tanpa tabel ini,
unlock harus ditebak dari `coin_transactions`, yang rapuh.

## 3. Aturan bisnis (diimplementasikan di usecase, bukan hanya dokumen)

1. **Episode gratis:** episode nomor `1..free_episode_count` per series
   GRATIS untuk semua penonton (termasuk tanpa VIP).
2. **Episode terkunci:** episode di atas `free_episode_count` TERKUNCI
   sampai dibuka dengan koin (`coin_price`) atau VIP aktif.
3. **VIP membuka semua:** selama ada `VipSubscription` berstatus aktif
   dan `ends_at` belum lewat, semua episode terbuka tanpa potong koin.
4. **VIP berakhir = kembali terkunci:** bila `ends_at` lewat, episode
   berbayar kembali terkunci kecuali sudah pernah di-unlock dengan koin.
   Status `active` yang kedaluwarsa diperlakukan tidak aktif saat dibaca
   (lazy expiry) dan tidak membuka episode apa pun.
5. **Unlock memotong saldo:** unlock mengurangi `CoinWallet.balance`
   sebesar `coin_price`, mencatat `CoinTransaction` tipe `unlock`
   (negatif) + `balance_after`, dan mencatat `episode_unlocks`.
   Semua langkah dalam satu transaksi database.
6. **Saldo kurang = error jelas:** bila saldo < harga, unlock GAGAL,
   saldo tidak berubah, pesan: "Saldo koin tidak cukup …".
7. **Check-in harian sekali/hari:** klaim kedua di hari yang sama GAGAL
   dengan pesan jelas. Dijaga oleh UNIQUE(user_id, task_id, claim_date)
   plus pengecekan di usecase, dalam satu transaksi (klaim + tambah
   saldo + transaksi koin).
8. **Unlock yang sudah ada:** episode gratis / sudah di-unlock / sudah
   terbuka karena VIP tidak bisa di-unlock lagi — masing-masing
   mengembalikan error yang berbeda dan jelas, bukan potong saldo diam-diam.
9. **Lanjutkan Nonton:** dari `WatchProgress`, diurutkan
   `last_watched_at` terbaru, disertai data episode + series.
10. **Progres hanya untuk episode yang bisa ditonton:** menyimpan
    progres episode terkunci ditolak dengan error jelas.
11. **Video terkunci disembunyikan:** episode terkunci yang diminta
    pengguna biasa mengembalikan metadata TANPA `video_url`
    (kosong) dan flag `locked: true`. Admin tidak terkena aturan ini.
12. **Admin mengelola semua via web (API):** hanya role `admin` yang
    boleh CRUD series, episode, genre, dan coin-packages. Endpoint
    publik hanya membaca data `published` + paket aktif.
13. **Komentar/suka/bookmark:** butuh login. Bookmark & like bersifat
    idempotent (POST ganda tidak membuat duplikat; DELETE yang tidak
    ada tetap sukses).
14. **Tugas non-check-in di fase ini:** `watch_ad` dan `finish_series`
    hanya tampil sebagai katalog tugas. Klaimnya belum dibuka karena
    iklan beneran dan verifikasi "series selesai" belum ada di fase ini
    — klaim selain check-in mengembalikan error jelas, bukan koin gratis.

## 4. Kontrak login (web admin & mobile SAMA)

Satu endpoint untuk kedua klien — tidak ada endpoint login terpisah per klien:

```
POST /api/v1/auth/login   { "email": "...", "password": "..." }
-> { "success": true, "data": { "token": "<JWT>", "user": { "id": ..., "name": "...", "email": "...", "role": "viewer"|"admin", ... } } }
```

- `user` SELALU memuat `role`; JWT claims SELALU memuat `role` yang sama
  (`sub` = user id, `role` = role, `exp` = 7 hari).
- Guard admin (CRUD admin + upload) diuji dari token saja: tanpa token
  = 401, token viewer = 403, token admin = lolos.
- Bila backend belum terhubung / token tidak valid / role bukan admin,
  endpoint mengembalikan error JSON yang jelas (401/403) — TIDAK PERNAH
  mengembalikan sukses palsu, khususnya untuk upload. Klien (web/mobile)
  wajib menampilkan pesan error itu, bukan berpura-pura upload berhasil.

## 5. Media upload admin (tahap staging/preview)

Backend ini API SAJA — tidak ada halaman HTML. Admin ada di dua klien
(web admin desktop + mode Admin di aplikasi Flutter); keduanya memakai
endpoint yang sama di bawah ini.

### Endpoint upload (guard: JWT wajib + role=admin)

| Endpoint | Field multipart | Hasil |
|---|---|---|
| `POST /api/v1/admin/series/{id}/cover` | `file` (gambar) | update `cover_url` series |
| `POST /api/v1/admin/episodes/{id}/video` | `file` (video) | update `video_url` episode |
| `POST /api/v1/admin/episodes/{id}/subtitle` | `file` (subtitle) | update `subtitle_url` episode |
| `POST /api/v1/admin/episodes/{id}/thumbnail` | `file` (gambar) | update `thumbnail_url` episode |

Guard: tanpa token → 401; token viewer → 403; token admin → diproses.
Respons sukses: `{ "success": true, "data": { "url": "media/...", "<field>": "media/..." } }`.

### Validasi & penyimpanan tahap ini

- Tipe file: gambar `png/jpg/jpeg/webp`, video `mp4/mov/mkv`,
  subtitle `vtt/srt` (validasi ekstensi + sanity check Content-Type multipart).
- Batas ukuran: gambar 5 MB, subtitle 5 MB, video 500 MB.
- Nama file diamankan: path traversal dibuang (`path.Base` + karakter
  tidak aman diganti `_`), nama dibuat unik dengan timestamp nano.
- Lokasi simpan (env `MEDIA_DIR`, deploy VPS: `/opt/dracin-reels-backend/media`):
  cover → `media/series/`, video/subtitle/thumbnail → `media/episodes/`.
- Nilai yang disimpan di `cover_url`/`video_url`/`subtitle_url`/`thumbnail_url`
  adalah path relatif terhadap base API, mis. `media/series/12-cover-....png`.
  Klien memintanya sebagai `<api-base>/media/series/...`
  (lewat proxy: `http://103.147.33.11:8095/dracin-api/media/series/...`).
- Asal file: field-field tersebut bisa berisi (a) URL eksternal/placeholder
  (seed awal memakai placeholder `placehold.co`/`example.com`), atau
  (b) path upload admin seperti di atas. API tidak membedakan keduanya
  selain dari isi stringnya.

### Dua sumber `video_url` (disetujui uwa: link eksternal "aman")

`Episode.video_url` mendukung DUA sumber, dan klien (web admin + mobile)
wajib menyediakan kedua pilihan di formulir episode — tidak boleh
mewajibkan upload bila link sudah diisi:

1. **File upload** (multipart admin, endpoint di atas): file disimpan di
   media VPS tahap ini, `video_url` diisi path relatif `media/episodes/...`.
2. **Tautan eksternal yang ditempel admin**: `video_url` diisi URL http(s)
   LANGSUNG ke file video/playlist (mis. dari object storage/CDN milik
   sendiri), TANPA file upload — cukup lewat CRUD episode biasa
   (`POST/PUT /api/v1/admin/episodes` dengan `video_url` terisi).

Aturan prioritas & validasi (diimplementasikan di usecase, bukan hanya dokumen):
- Salah satu cukup: tidak wajib upload bila `video_url` sudah diisi, dan
  tidak wajib `video_url` bila video akan diupload (boleh kosong saat
  episode dibuat, lalu diupload belakangan).
- Bila keduanya diisi (episode sudah punya `video_url` eksternal lalu
  admin mengupload file), **upload yang menang**: endpoint upload menimpa
  `video_url` dengan URL file hasil upload.
- Validasi link: `video_url` kosong = boleh; path relatif `media/...`
  (hasil upload sendiri) = boleh; selain itu HARUS URL http(s) valid
  dengan host — skema lain (`javascript:`, `file:`, `ftp:`, dsb.)
  DITOLAK dengan error 400 yang jelas.
- Pemutaran memakai URL apa pun sumbernya (API tidak membedakan sumber
  saat mengembalikan episode yang sudah terbuka).
- Media diserve backend lewat `GET /media/...` HANYA sebagai
  staging/preview admin — TANPA proteksi auth dan TANPA proteksi kunci
  episode di level file (proteksi kunci berlaku di level API episode).
- ⚠️ JUJUR: untuk tayang publik skala besar, penyimpanan HARUS pindah
  ke object storage + CDN (aturan lama reels: video jangan diserve dari
  VPS kecil untuk publik massal, dan URL media harus bertanda tangan/
  berbatas). Tahap ini BELUM itu — jangan jadikan URL `/media/...`
  sebagai distribusi publik massal.

## 6. Seed sampel (lihat `internal/infrastructure/database/seed/`)

- 1 series orisinal: **CINTA SANG PENGUASA QIN** — 5 episode,
  `free_episode_count = 2`, rating 4.8, genre Romansa, Balas Dendam,
  Fantasi, Misteri.
- Episode 1–2 gratis (`coin_price = 0`); episode 3 = 50 koin,
  4 = 80 koin, 5 = 100 koin.
- 4 paket koin sampel, 3 tugas hadiah (check-in 10 koin/hari).
- 1 admin + 1 viewer demo (kredensial demo HANYA di README, dan
  wajib diganti sebelum data asli masuk — lihat README).
- Viewer demo diberi saldo 200 koin agar alur unlock bisa dicoba
  (cukup untuk episode 3 + 4, tidak cukup untuk semuanya sekaligus).
