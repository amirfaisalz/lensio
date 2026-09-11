import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "../services/api";
import { OverviewPage } from "./OverviewPage";

describe("OverviewPage", () => {
	beforeEach(() => {
		localStorage.clear();
		vi.restoreAllMocks();
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

		render(<OverviewPage />);

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

		render(<OverviewPage />);

		await waitFor(() => {
			expect(
				screen.getByText("Metrics service temporarily unavailable"),
			).toBeDefined();
		});
	});

	it("tests synthetic fixture OCR execution in playground", async () => {
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

		vi.spyOn(api, "executeKTPOCR").mockResolvedValue(mockOcrResult);

		render(<OverviewPage />);

		await waitFor(() => {
			expect(screen.getByText("System Overview")).toBeDefined();
		});

		// Click "Load Synthetic Fixture"
		const fixtureBtn = screen.getByRole("button", {
			name: /Load Synthetic Fixture/i,
		});
		fireEvent.click(fixtureBtn);

		await waitFor(() => {
			expect(screen.getByText("Normalized Field Verification")).toBeDefined();
			expect(screen.getByText("3273012345670001")).toBeDefined();
			expect(screen.getByText("JOKO WIDODO SYNTHETIC")).toBeDefined();
			expect(screen.getByText(/99% Confidence/)).toBeDefined();
		});
	});

	it("handles file upload error in playground", async () => {
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

		render(<OverviewPage />);

		await waitFor(() => {
			expect(screen.getByText("System Overview")).toBeDefined();
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
