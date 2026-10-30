-- Dracin Reels — demo seed. Runs ONLY when the users table is empty
-- (see database.SeedIfEmpty). Demo passwords are in README and MUST be
-- changed before any real data is used.
-- NOTE: video/cover URLs are placeholders; real streaming/CDN is not in this phase.
-- Password hashes below are bcrypt ($2a$ prefix for Go compatibility):
--   admin  = Admin123!
--   viewer = Viewer123!

INSERT INTO users (name, email, password_hash, role) VALUES
  ('Admin Demo', 'admin@demo.dracin.local', '$2a$05$CgM.0sl/dNFmcSphSJOo5e809zVqtiNMWC3e1RXzwbkzDDiaAcNTq', 'admin'),
  ('Penonton Demo', 'viewer@demo.dracin.local', '$2a$05$/3KtJbCyjQCnGWyq45AnS.HZjt3/eRcTqaUXN1W1ACq.BVo9PXzMi', 'viewer')
ON CONFLICT (email) DO NOTHING;

INSERT INTO coin_wallets (user_id, balance)
SELECT id, CASE WHEN email = 'viewer@demo.dracin.local' THEN 200 ELSE 0 END
FROM users WHERE email IN ('admin@demo.dracin.local', 'viewer@demo.dracin.local')
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO genres (name, slug) VALUES
  ('Romansa', 'romansa'),
  ('Balas Dendam', 'balas-dendam'),
  ('Fantasi', 'fantasi'),
  ('Misteri', 'misteri')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO series (title, description, cover_url, status, total_episode, free_episode_count, rating) VALUES
  ('CINTA SANG PENGUASA QIN',
   'Kisah orisinal: seorang tabib muda yang menyamar menjadi pelayan istana demi mencari obat bagi adiknya, justru terjebak dalam perebutan takhta dan hati sang penguasa Qin. Drama pendek vertikal 5 episode.',
   'https://placehold.co/600x800?text=CINTA+SANG+PENGUASA+QIN',
   'published', 5, 2, 4.8)
ON CONFLICT DO NOTHING;

INSERT INTO series_genres (series_id, genre_id)
SELECT s.id, g.id FROM series s, genres g
WHERE s.title = 'CINTA SANG PENGUASA QIN'
  AND g.slug IN ('romansa', 'balas-dendam', 'fantasi', 'misteri')
ON CONFLICT DO NOTHING;

INSERT INTO episodes (series_id, number, title, description, video_url, subtitle_url, duration_seconds, coin_price, status, thumbnail_url)
SELECT s.id, e.number, e.title, e.description, e.video_url, NULL, e.duration_seconds, e.coin_price, 'published',
       'https://placehold.co/600x800?text=QIN+EP+' || e.number
FROM series s
JOIN (VALUES
  (1, 'Pertemuan di Pasar Qin', 'Tabib muda Lin Yue bertemu pria misterius di pasar Qin.', 'https://example.com/videos/qin-ep1.mp4', 95, 0),
  (2, 'Janji di Bawah Lentera', 'Sebuah janji di malam festival lentera mengubah takdir Lin Yue.', 'https://example.com/videos/qin-ep2.mp4', 110, 0),
  (3, 'Rahasia di Balik Tirai Istana', 'Lin Yue menyusup ke istana dan menemukan rahasia sang penguasa.', 'https://example.com/videos/qin-ep3.mp4', 105, 50),
  (4, 'Balas Dendam Sang Tabib', 'Masa lalu keluarga Lin Yue terungkap; ia harus memilih dendam atau ampunan.', 'https://example.com/videos/qin-ep4.mp4', 120, 80),
  (5, 'Takhta dan Hati', 'Di ambang perang saudara, cinta dan takhta dipertaruhkan sekaligus.', 'https://example.com/videos/qin-ep5.mp4', 135, 100)
) AS e(number, title, description, video_url, duration_seconds, coin_price) ON true
WHERE s.title = 'CINTA SANG PENGUASA QIN'
ON CONFLICT (series_id, number) DO NOTHING;

INSERT INTO coin_packages (name, coin_amount, price_idr, bonus_coins, active) VALUES
  ('Paket 100 Koin', 100, 15000, 0, true),
  ('Paket 550 Koin', 550, 75000, 50, true),
  ('Paket 1200 Koin', 1200, 150000, 200, true),
  ('Paket 3000 Koin', 3000, 350000, 700, true)
ON CONFLICT DO NOTHING;

INSERT INTO reward_tasks (name, type, coin_reward, daily_limit) VALUES
  ('Check-in Harian', 'checkin', 10, 1),
  ('Tonton Iklan', 'watch_ad', 5, 5),
  ('Selesaikan Satu Series', 'finish_series', 50, 1)
ON CONFLICT DO NOTHING;
