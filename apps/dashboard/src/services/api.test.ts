import { beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";

describe("ApiClient", () => {
	beforeEach(() => {
		localStorage.clear();
		api.setApiKey(null);
		vi.restoreAllMocks();
	});

	it("manages API key in memory without persisting to localStorage", () => {
		expect(api.getApiKey()).toBeNull();
		api.setApiKey("lensio_live_abc123");
		expect(api.getApiKey()).toBe("lensio_live_abc123");
		expect(localStorage.getItem("lensio_api_key")).toBeNull();

		api.setApiKey(null);
		expect(api.getApiKey()).toBeNull();
		expect(localStorage.getItem("lensio_api_key")).toBeNull();
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
			key: "lensio_live_secret_key_token",
			prefix: "lensio_live_secr",
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

		expect(res.key).toBe("lensio_live_secret_key_token");
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

	it("executes SIM OCR multipart request", async () => {
		const mockSim = {
			id: "ocr_sim_123",
			document_type: "sim" as const,
			status: "completed",
			confidence: 0.98,
			processing: { latency_ms: 120 },
			data: {
				nomor_sim: "123456789012",
				golongan: "A",
				nama: "BUDI SANTOSO",
				alamat: "JL. SUDIRMAN NO. 1",
				rt_rw: "001/002",
				kelurahan: "GELORA",
				kecamatan: "TANAH ABANG",
				kota: "JAKARTA PUSAT",
				pekerjaan: "KARYAWAN",
				tempat_lahir: "JAKARTA",
				tanggal_lahir: "1990-01-01",
				jenis_kelamin: "PRIA",
				golongan_darah: "O",
				masa_berlaku: "2029-01-01",
				polda: "POLDA METRO JAYA",
			},
			field_confidence: {
				nomor_sim: 0.99,
				nama: 0.98,
			},
		};

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: true,
			json: async () => mockSim,
		} as Response);

		const dummyBlob = new Blob(["fake image"], { type: "image/jpeg" });
		const result = await api.executeSIMOCR(dummyBlob);

		expect(result.data.nomor_sim).toBe("123456789012");
		expect(result.data.golongan).toBe("A");
		expect(result.status).toBe("completed");
	});

	it("executes Passport OCR multipart request", async () => {
		const mockPassport = {
			id: "ocr_pass_123",
			document_type: "passport" as const,
			status: "completed",
			confidence: 0.99,
			processing: { latency_ms: 130 },
			data: {
				passport_number: "X1234567",
				full_name: "BUDI SANTOSO",
				nationality: "IDN",
				date_of_birth: "1990-01-01",
				gender: "M",
				expiry_date: "2030-01-01",
				issuing_country: "IDN",
				issuing_office: "JAKARTA SELATAN",
				mrz_line1: "P<IDNSANTOSO<<BUDI<<<<<<<<<<<<<<<<<<<<<<<<<<",
				mrz_line2: "X1234567<7IDN9001011M3001019<<<<<<<<<<<<<<<2",
			},
			field_confidence: {
				passport_number: 0.99,
				full_name: 0.99,
			},
		};

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: true,
			json: async () => mockPassport,
		} as Response);

		const dummyBlob = new Blob(["fake image"], { type: "image/jpeg" });
		const result = await api.executePassportOCR(dummyBlob);

		expect(result.data.passport_number).toBe("X1234567");
		expect(result.data.full_name).toBe("BUDI SANTOSO");
		expect(result.status).toBe("completed");
	});

	it("registers user successfully", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: true,
			status: 201,
			json: async () => ({
				status: "pending_verification",
				email: "budi@fintech.id",
				verification_token: "tok-123",
				message: "Registrasi berhasil.",
			}),
		} as Response);

		const res = await api.register({
			full_name: "Budi Santoso",
			email: "budi@fintech.id",
			password: "password123",
		});
		expect(res.status).toBe("pending_verification");
		expect(res.email).toBe("budi@fintech.id");
	});

	it("verifies user email successfully", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: true,
			status: 200,
			json: async () => ({
				status: "verified",
				message: "Email berhasil diverifikasi!",
			}),
		} as Response);

		const res = await api.verifyEmail({
			email: "budi@fintech.id",
			token: "tok-123",
		});
		expect(res.status).toBe("verified");
	});

	it("logs in with password and handles tokens", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: true,
			status: 200,
			json: async () => ({
				status: "authenticated",
				user: {
					id: "u-1",
					email: "budi@fintech.id",
					full_name: "Budi Santoso",
					role: "member",
				},
				organization: null,
			}),
		} as Response);

		const res = await api.login({
			email: "budi@fintech.id",
			password: "password123",
		});
		expect(res.status).toBe("authenticated");
		expect(res.user.email).toBe("budi@fintech.id");
	});

	it("creates organization successfully", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: true,
			status: 201,
			json: async () => ({
				organization: {
					id: "org-acme",
					name: "Acme Corp",
					slug: "acme-corp",
					plan_code: "free",
				},
			}),
		} as Response);

		const res = await api.createOrganization({
			name: "Acme Corp",
			plan_code: "free",
		});
		expect(res.organization.id).toBe("org-acme");
		expect(res.organization.name).toBe("Acme Corp");
	});

	it("fetches current user and handles logout", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: true,
			status: 200,
			json: async () => ({
				user: {
					id: "u-1",
					email: "user@example.com",
					full_name: "Test User",
					roles: ["developer"],
				},
				organization: {
					id: "org-1",
					name: "My Org",
					slug: "my-org",
					plan_code: "free",
				},
			}),
		} as Response);

		const userRes = await api.fetchCurrentUser();
		expect(userRes.user?.email).toBe("user@example.com");
		expect(userRes.organization?.name).toBe("My Org");

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: true,
			status: 200,
			json: async () => ({
				status: "ok",
				message: "Logged out",
			}),
		} as Response);

		const logoutRes = await api.logout();
		expect(logoutRes.status).toBe("ok");
	});
});
