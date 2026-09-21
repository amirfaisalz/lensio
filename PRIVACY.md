# Lensio Privacy Notice (UU PDP No. 27/2022)

> Ringkas, jujur, dan operasional. Bukan nasihat hukum.

## Data yang diproses
- **Gambar dokumen** (KTP, SIM, Passport, NPWP, KK, Invoice) via `multipart/form-data`, max 5MB, JPEG/PNG/WebP. Diproses **di RAM saja**, tidak ditulis ke disk/blob. Buffer dilepas setelah respons.
- **Hasil ekstraksi** dikembalikan ke pemanggil dan **tidak disimpan** (yang disimpan hanya metadata non-PII: `record_id, org_id, confidence, latency_ms, doc_type, status`).
- **Pengecualian: `Idempotency-Key`.** Jika pemanggil mengirim header ini, respons (termasuk field hasil ekstraksi) disimpan agar retry mengembalikan hasil yang sama. Sejak versi ini body tersebut **dienkripsi AES-256-GCM** dengan kunci turunan `SESSION_SECRET` sebelum masuk PostgreSQL, dan **kedaluwarsa 1 jam** (sebelumnya 24 jam, plaintext). Tanpa header ini tidak ada hasil ekstraksi yang menyentuh database. Untuk retensi nol, jangan kirim `Idempotency-Key`.
- **Log** hanya berisi `request_id, trace_id, org_id, api_key_id, latency, status`. NIK 16-digit selalu menjadi `[REDACTED]`.

## Transfer lintas negara
Provider default `gemini_flash` mengirim buffer gambar ke **Google AI Studio / Gemini API** (infrastruktur di luar Indonesia) untuk inferensi vision. Dasar UU PDP Pasal 56: gunakan Lensio hanya dengan persetujuan subjek data + kontrak/DPA yang memadai, atau jalankan dengan `OCR_PROVIDER=mock` / provider dalam negeri untuk evaluasi tanpa transfer.

## Dasar & persetujuan
Operator API (Anda) adalah Pengendali Data: wajib memperoleh persetujuan subjek, membatasi tujuan pada verifikasi identitas yang dinyatakan, dan menyediakan mekanisme keberatan/penghapusan di sisi Anda.

## Retensi
- Gambar: `0` — tidak disimpan.
- Cache idempotency (`idempotency_keys.response_body`): **1 jam**, terenkripsi. Hanya terisi bila pemanggil memakai `Idempotency-Key`.
- Metadata `ocr_requests`: hapus/anonimkan maksimal **90 hari** (atur cron di sisi operator).
- `usage_records` agregat kuota: **12 bulan**, lalu agregat.
- `audit_logs` administratif: **12 bulan**.
- Backup mengikuti retensi yang sama.

## Hak subjek
Karena Lensio tidak menyimpan PII di luar cache idempotency berdurasi 1 jam di atas, permintaan akses/hapus diajukan ke operator aplikasi pemanggil. Kontak DPO operator wajib dicantumkan di aplikasi Anda, bukan di repo ini.

## Validasi ≠ verifikasi kependudukan
Semua validator kami **struktural**, bukan verifikasi ke instansi penerbit:
- `ValidateNIK`: 16 digit, kode provinsi/kabupaten/kecamatan, tanggal + offset 40. **Bukan** verifikasi Dukcapil.
- `ValidateMRZTD3`: check digit ICAO 9303 (nomor paspor, lahir, kedaluwarsa, personal number, composite). Membuktikan MRZ konsisten secara matematis, **bukan** bahwa paspornya asli atau berlaku.
- `ValidateNPWP`: Luhn atas 8 digit pertama. DJP tidak pernah menerbitkan algoritma ini — ini konvensi ekosistem, **bukan** spesifikasi. Lolos = plausibel secara struktur, bukan terbukti terdaftar.
- `ValidateInvoice`: aritmetika pajak dan tarif PPN statutori (12%/11%/10%, atau 0 untuk non-PKP/ekspor/bebas). **Bukan** verifikasi e-Faktur ke DJP.

Jangan mengklaim keaslian orang atau dokumen berdasarkan skor confidence.
