# Lensio Privacy Notice (UU PDP No. 27/2022)

> Ringkas, jujur, dan operasional. Bukan nasihat hukum.

## Data yang diproses
- **Gambar dokumen** (KTP/SIM) via `multipart/form-data`, max 5MB, JPEG/PNG/WebP. Diproses **di RAM saja**, tidak ditulis ke disk/blob. Buffer dilepas setelah respons.
- **Hasil ekstraksi** dikembalikan ke pemanggil dan **tidak disimpan** (yang disimpan hanya metadata non-PII: `record_id, org_id, confidence, latency_ms, doc_type, status`).
- **Log** hanya berisi `request_id, trace_id, org_id, api_key_id, latency, status`. NIK 16-digit selalu menjadi `[REDACTED]`.

## Transfer lintas negara
Provider default `gemini_flash` mengirim buffer gambar ke **Google AI Studio / Gemini API** (infrastruktur di luar Indonesia) untuk inferensi vision. Dasar UU PDP Pasal 56: gunakan Lensio hanya dengan persetujuan subjek data + kontrak/DPA yang memadai, atau jalankan dengan `OCR_PROVIDER=mock` / provider dalam negeri untuk evaluasi tanpa transfer.

## Dasar & persetujuan
Operator API (Anda) adalah Pengendali Data: wajib memperoleh persetujuan subjek, membatasi tujuan pada verifikasi identitas yang dinyatakan, dan menyediakan mekanisme keberatan/penghapusan di sisi Anda.

## Retensi
- Gambar: `0` — tidak disimpan.
- Metadata `ocr_requests`: hapus/anonimkan maksimal **90 hari** (atur cron di sisi operator).
- `usage_records` agregat kuota: **12 bulan**, lalu agregat.
- `audit_logs` administratif: **12 bulan**.
- Backup mengikuti retensi yang sama.

## Hak subjek
Karena Lensio tidak menyimpan PII, permintaan akses/hapus diajukan ke operator aplikasi pemanggil. Kontak DPO operator wajib dicantumkan di aplikasi Anda, bukan di repo ini.

## Validasi ≠ verifikasi kependudukan
`ValidateNIK` hanya cek struktur (16 digit, kode provinsi, tanggal+offset 40). **Bukan** verifikasi ke Dukcapil. Jangan mengklaim keaslian orang berdasarkan skor confidence.
