import { beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";

describe("ApiClient", () => {
	beforeEach(() => {
		localStorage.clear();
		api.setApiKey(null);
		vi.restoreAllMocks();
	});

	it("manages API key in memory and localStorage", () => {
		expect(api.getApiKey()).toBeNull();
		api.setApiKey("nusa_live_abc123");
		expect(api.getApiKey()).toBe("nusa_live_abc123");
		expect(localStorage.getItem("nusaid_api_key")).toBe("nusa_live_abc123");

		api.setApiKey(null);
		expect(api.getApiKey()).toBeNull();
		expect(localStorage.getItem("nusaid_api_key")).toBeNull();
	});

	it("fetches usage summary successfully", async () => {
		const mockSummary = {
			total_requests: 100,
			success_count: 98,
			error_count: 2,
			quota_limit: 1000,
			quota_remaining: 900,
			p95_latency_ms: 150,
			rate_limit_violations: 0,
			billing_cycle_reset: "2026-10-01T00:00:00Z",
		};

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: true,
			json: async () => mockSummary,
		} as Response);

		const data = await api.fetchUsageSummary();
		expect(data.total_requests).toBe(100);
		expect(data.p95_latency_ms).toBe(150);
	});

	it("handles structured API error responses", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: false,
			status: 401,
			json: async () => ({
				error: {
					code: "invalid_api_key",
					message: "The provided API key is invalid",
				},
			}),
		} as Response);

		await expect(api.fetchUsageSummary()).rejects.toThrow(
			"The provided API key is invalid",
		);
	});

	it("creates API key and returns plaintext token", async () => {
		const mockCreated = {
			id: "key-1",
			org_id: "org-1",
			name: "New Test Key",
			key: "nusa_live_secret_key_token",
			prefix: "nusa_live_secr",
			scopes: ["ocr:write"],
			environment: "live",
			expires_at: null,
			created_at: "2026-09-11T00:00:00Z",
		};

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: true,
			json: async () => mockCreated,
		} as Response);

		const res = await api.createAPIKey({
			name: "New Test Key",
			environment: "live",
			scopes: ["ocr:write"],
		});

		expect(res.key).toBe("nusa_live_secret_key_token");
		expect(res.name).toBe("New Test Key");
	});

	it("revokes API key successfully", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: true,
			json: async () => ({
				message: "API key successfully revoked",
				id: "key-1",
			}),
		} as Response);

		const res = await api.revokeAPIKey("key-1");
		expect(res.id).toBe("key-1");
	});

	it("executes KTP OCR multipart request", async () => {
		const mockKtp = {
			id: "ocr_123",
			status: "completed",
			confidence: 0.98,
			latency_ms: 120,
			data: {
				nik: "3171012345670001",
				nama: "BUDI SANTOSO",
				tempat_lahir: "JAKARTA",
				tanggal_lahir: "1990-01-01",
				jenis_kelamin: "LAKI-LAKI",
				alamat: "JL. SUDIRMAN NO. 1",
				rt_rw: "001/002",
				kelurahan: "GELORA",
				kecamatan: "TANAH ABANG",
				agama: "ISLAM",
				status_perkawinan: "KAWIN",
				pekerjaan: "KARYAWAN SWASTA",
				kewarganegaraan: "WNI",
			},
			field_confidence: {
				nik: 0.99,
				nama: 0.98,
			},
		};

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: true,
			json: async () => mockKtp,
		} as Response);

		const dummyBlob = new Blob(["fake image"], { type: "image/jpeg" });
		const result = await api.executeKTPOCR(dummyBlob);

		expect(result.data.nik).toBe("3171012345670001");
		expect(result.status).toBe("completed");
	});
});
