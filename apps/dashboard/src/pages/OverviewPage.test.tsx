import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../context/AuthContext";
import { api } from "../services/api";
import { OverviewPage } from "./OverviewPage";

const TEST_ORG = {
	id: "org-test-uuid",
	name: "Acme Corp",
	slug: "acme",
	planCode: "free",
};

const renderWithAuth = (ui: React.ReactElement) => {
	return render(<AuthProvider>{ui}</AuthProvider>);
};

describe("OverviewPage", () => {
	beforeEach(() => {
		api.setApiKey(null);
		localStorage.clear();
		vi.restoreAllMocks();
	});

	it("does not render the playground when currentOrg is null", async () => {
		const mockSummary = {
			total_requests: 0,
			success_count: 0,
			error_count: 0,
			quota_limit: 100,
			quota_remaining: 100,
			p95_latency_ms: 0,
			rate_limit_violations: 0,
			billing_cycle_reset: "2026-10-01T00:00:00Z",
		};

		vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
			const url = String(input);
			if (url.includes("/api/v1/usage")) {
				return { ok: true, json: async () => mockSummary } as Response;
			}
			return { ok: true, json: async () => ({}) } as Response;
		});

		renderWithAuth(<OverviewPage />);

		await waitFor(() => {
			expect(screen.getByText("System Overview")).toBeDefined();
		});

		// Playground must be hidden when there is no organization
		expect(screen.queryByText("Live KTP OCR Playground")).toBeNull();
		expect(
			screen.queryByRole("button", { name: /Load Synthetic Fixture/i }),
		).toBeNull();
		expect(
			screen.getByText(
				"Selamat Datang di Lensio! Buat organisasi Anda terlebih dahulu",
			),
		).toBeDefined();
	});

	it("renders loading skeleton and then metric cards", async () => {
		const mockSummary = {
			total_requests: 1250,
			success_count: 1200,
			error_count: 50,
			quota_limit: 5000,
			quota_remaining: 3750,
			p95_latency_ms: 210,
			rate_limit_violations: 4,
			billing_cycle_reset: "2026-10-01T00:00:00Z",
		};

		vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
			const url = String(input);
			if (url.includes("/api/v1/usage")) {
				return { ok: true, json: async () => mockSummary } as Response;
			}
			return { ok: true, json: async () => ({}) } as Response;
		});

		renderWithAuth(<OverviewPage />);

		await waitFor(() => {
			expect(screen.getAllByText("1,250").length).toBe(2);
			expect(screen.getByText("96.0%")).toBeDefined();
			expect(screen.getByText("210ms")).toBeDefined();
			expect(screen.getByText("25% Used")).toBeDefined();
			expect(screen.getByText(/of 5,000 requests used/)).toBeDefined();
		});
	});

	it("displays error banner when loading metrics fails", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: false,
			status: 500,
			json: async () => ({
				error: { message: "Metrics service temporarily unavailable" },
			}),
		} as Response);

		renderWithAuth(<OverviewPage />);

		await waitFor(() => {
			expect(
				screen.getByText("Metrics service temporarily unavailable"),
			).toBeDefined();
		});
	});

	it("tests synthetic fixture OCR execution in playground when organization and apiKey exist", async () => {
		localStorage.setItem("lensio_current_org", JSON.stringify(TEST_ORG));
		api.setApiKey("lensio_live_testkey123");

		const mockSummary = {
			total_requests: 10,
			success_count: 10,
			error_count: 0,
			quota_limit: 1000,
			quota_remaining: 990,
			p95_latency_ms: 120,
			rate_limit_violations: 0,
			billing_cycle_reset: "2026-10-01T00:00:00Z",
		};

		const mockOcrResult = {
			id: "ocr_play_1",
			status: "completed",
			confidence: 0.99,
			latency_ms: 115,
			data: {
				nik: "3273012345670001",
				nama: "JOKO WIDODO SYNTHETIC",
				tempat_lahir: "SURAKARTA",
				tanggal_lahir: "1961-06-21",
				jenis_kelamin: "LAKI-LAKI",
				alamat: "JL. VETERAN NO. 1",
				rt_rw: "001/001",
				kelurahan: "MANAHAN",
				kecamatan: "BANJARSARI",
				agama: "ISLAM",
				status_perkawinan: "KAWIN",
				pekerjaan: "PEGAWAI NEGERI",
				kewarganegaraan: "WNI",
			},
			field_confidence: {
				nik: 1.0,
				nama: 0.99,
			},
		};

		vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
			const url = String(input);
			if (url.includes("/api/v1/usage")) {
				return { ok: true, json: async () => mockSummary } as Response;
			}
			return { ok: true, json: async () => ({}) } as Response;
		});

		const ocrSpy = vi
			.spyOn(api, "executeKTPOCR")
			.mockResolvedValue(mockOcrResult);

		renderWithAuth(<OverviewPage />);

		await waitFor(() => {
			expect(screen.getByText("System Overview")).toBeDefined();
			expect(screen.getByText("Live KTP OCR Playground")).toBeDefined();
		});

		// Click "Load Synthetic Fixture"
		const fixtureBtn = screen.getByRole("button", {
			name: /Load Synthetic Fixture/i,
		});
		fireEvent.click(fixtureBtn);

		await waitFor(() => {
			expect(ocrSpy).toHaveBeenCalledTimes(1);
			expect(screen.getByText("Normalized Field Verification")).toBeDefined();
			expect(screen.getByText("3273012345670001")).toBeDefined();
			expect(screen.getByText("JOKO WIDODO SYNTHETIC")).toBeDefined();
			expect(screen.getByText(/99% Confidence/)).toBeDefined();
		});
	});

	it("shows warning and blocks execution when organization exists but apiKey is missing", async () => {
		localStorage.setItem("lensio_current_org", JSON.stringify(TEST_ORG));
		// No API key set in localStorage

		const mockSummary = {
			total_requests: 0,
			success_count: 0,
			error_count: 0,
			quota_limit: 100,
			quota_remaining: 100,
			p95_latency_ms: 0,
			rate_limit_violations: 0,
			billing_cycle_reset: "2026-10-01T00:00:00Z",
		};

		vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
			const url = String(input);
			if (url.includes("/api/v1/usage")) {
				return { ok: true, json: async () => mockSummary } as Response;
			}
			return { ok: true, json: async () => ({}) } as Response;
		});

		const ocrSpy = vi.spyOn(api, "executeKTPOCR");

		renderWithAuth(<OverviewPage />);

		await waitFor(() => {
			expect(screen.getByText("Live KTP OCR Playground")).toBeDefined();
			expect(
				screen.getByText(
					/API Key diperlukan untuk menguji OCR di playground ini/,
				),
			).toBeDefined();
		});

		// Try clicking "Load Synthetic Fixture"
		const fixtureBtn = screen.getByRole("button", {
			name: /Load Synthetic Fixture/i,
		});
		fireEvent.click(fixtureBtn);

		// Must NOT call api.executeKTPOCR and must display error banner
		await waitFor(() => {
			expect(ocrSpy).not.toHaveBeenCalled();
			expect(
				screen.getByText(/API Key aktif diperlukan untuk menjalankan OCR/),
			).toBeDefined();
		});
	});

	it("handles file upload error in playground", async () => {
		localStorage.setItem("lensio_current_org", JSON.stringify(TEST_ORG));
		api.setApiKey("lensio_live_testkey123");

		const mockSummary = {
			total_requests: 5,
			success_count: 5,
			error_count: 0,
			quota_limit: 1000,
			quota_remaining: 995,
			p95_latency_ms: 150,
			rate_limit_violations: 0,
			billing_cycle_reset: "2026-10-01T00:00:00Z",
		};

		vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
			const url = String(input);
			if (url.includes("/api/v1/usage")) {
				return { ok: true, json: async () => mockSummary } as Response;
			}
			return { ok: true, json: async () => ({}) } as Response;
		});

		vi.spyOn(api, "executeKTPOCR").mockRejectedValue(
			new Error("Image blur score too low for OCR extraction"),
		);

		renderWithAuth(<OverviewPage />);

		await waitFor(() => {
			expect(screen.getByText("System Overview")).toBeDefined();
			expect(screen.getByText("Live KTP OCR Playground")).toBeDefined();
		});

		const fileInput = document.getElementById(
			"ktp-file-input",
		) as HTMLInputElement;
		expect(fileInput).toBeDefined();

		const file = new File(["dummy content"], "test_ktp.jpg", {
			type: "image/jpeg",
		});
		fireEvent.change(fileInput, { target: { files: [file] } });

		await waitFor(() => {
			expect(
				screen.getByText("Image blur score too low for OCR extraction"),
			).toBeDefined();
		});
	});
});
