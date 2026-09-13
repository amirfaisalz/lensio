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

const renderWithAuth = (
	ui: React.ReactElement,
	options?: {
		initialOrg?: typeof TEST_ORG | null;
		initialApiKey?: string | null;
	},
) => {
	return render(
		<AuthProvider
			initialOrg={options?.initialOrg}
			initialApiKey={options?.initialApiKey}
		>
			{ui}
		</AuthProvider>,
	);
};

describe("OverviewPage", () => {
	beforeEach(() => {
		api.setApiKey(null);
		localStorage.clear();
		vi.restoreAllMocks();
	});

	it("renders onboarding callout when currentOrg is null", async () => {
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

		const onNavigate = vi.fn();
		renderWithAuth(<OverviewPage onNavigate={onNavigate} />);

		await waitFor(() => {
			expect(screen.getByText("System Overview")).toBeDefined();
		});

		expect(
			screen.getByText(
				"Selamat Datang di Lensio! Buat organisasi Anda terlebih dahulu",
			),
		).toBeDefined();

		const btn = screen.getByRole("button", { name: /Buat Organisasi/i });
		fireEvent.click(btn);
		expect(onNavigate).toHaveBeenCalledWith("keys");
	});

	it("renders loading skeleton and then metric cards, quota progress, and quick action cards", async () => {
		const mockSummary = {
			total_requests: 1250,
			success_count: 1200,
			error_count: 50,
			quota_limit: 5000,
			quota_remaining: 3750,
			p95_latency_ms: 210,
			p95_ocr_latency_ms: 12540,
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

		const onNavigate = vi.fn();
		renderWithAuth(<OverviewPage onNavigate={onNavigate} />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		await waitFor(() => {
			expect(screen.getAllByText("1,250").length).toBe(2);
			expect(screen.getByText("96.0%")).toBeDefined();
			expect(screen.getByText("210ms")).toBeDefined();
			expect(screen.getByText("Upstream AI Latency")).toBeDefined();
			expect(screen.getByText("12,540ms")).toBeDefined();
			expect(screen.getByText("25% Used")).toBeDefined();
			expect(screen.getByText(/of 5,000 requests used/)).toBeDefined();
			expect(screen.getByText("4")).toBeDefined();
		});

		// Verify Quick Action cards
		expect(screen.getByText("Live OCR Playground")).toBeDefined();
		expect(screen.getByText("Kelola API Key")).toBeDefined();
		expect(screen.getByText("Dokumentasi API")).toBeDefined();

		// Click "Buka Playground"
		fireEvent.click(screen.getByRole("button", { name: /Buka Playground/i }));
		expect(onNavigate).toHaveBeenCalledWith("playground");

		// Click "Atur Kunci API"
		fireEvent.click(screen.getByRole("button", { name: /Atur Kunci API/i }));
		expect(onNavigate).toHaveBeenCalledWith("keys");

		// Click "Lihat Dokumentasi"
		fireEvent.click(screen.getByRole("button", { name: /Lihat Dokumentasi/i }));
		expect(onNavigate).toHaveBeenCalledWith("docs");
	});

	it("displays error banner when loading metrics fails and allows retrying", async () => {
		const fetchSpy = vi
			.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce({
				ok: false,
				status: 500,
				json: async () => ({
					error: { message: "Metrics service temporarily unavailable" },
				}),
			} as Response)
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					total_requests: 10,
					success_count: 10,
					error_count: 0,
					quota_limit: 100,
					quota_remaining: 90,
					p95_latency_ms: 100,
					rate_limit_violations: 0,
					billing_cycle_reset: "2026-10-01T00:00:00Z",
				}),
			} as Response);

		renderWithAuth(<OverviewPage />);

		await waitFor(() => {
			expect(
				screen.getByText("Metrics service temporarily unavailable"),
			).toBeDefined();
		});

		// Click Retry button
		fireEvent.click(screen.getByRole("button", { name: /Retry/i }));

		await waitFor(() => {
			expect(fetchSpy).toHaveBeenCalledTimes(2);
		});
	});

	it("renders first API key banner when the organization has no keys yet", async () => {
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

		const onNavigate = vi.fn();
		renderWithAuth(<OverviewPage onNavigate={onNavigate} />, {
			initialOrg: TEST_ORG,
		});

		await waitFor(() => {
			expect(
				screen.getByText(
					`Welcome to ${TEST_ORG.name}! Create your first API Key`,
				),
			).toBeDefined();
		});

		const createKeyBtn = screen.getByRole("button", {
			name: /Create API Key/i,
		});
		fireEvent.click(createKeyBtn);
		expect(onNavigate).toHaveBeenCalledWith("keys");
	});

	it("hides the first API key banner once the organization has keys", async () => {
		const mockSummary = {
			total_requests: 10,
			success_count: 10,
			error_count: 0,
			quota_limit: 100,
			quota_remaining: 90,
			p95_latency_ms: 100,
			p95_ocr_latency_ms: 0,
			rate_limit_violations: 0,
			billing_cycle_reset: "2026-10-01T00:00:00Z",
		};
		let keys: unknown[] = [];

		vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
			const url = String(input);
			if (url.includes("/api/v1/auth/api-keys")) {
				return { ok: true, json: async () => ({ data: keys }) } as Response;
			}
			if (url.includes("/api/v1/usage")) {
				return { ok: true, json: async () => mockSummary } as Response;
			}
			return { ok: true, json: async () => ({}) } as Response;
		});

		renderWithAuth(<OverviewPage />, {
			initialOrg: TEST_ORG,
		});

		await waitFor(() => {
			expect(screen.getByText(/Create your first API Key/)).toBeDefined();
		});

		keys = [
			{
				id: "key-1",
				name: "Existing Key",
				prefix: "lensio_live_abcd",
				masked_key: "lensio_live_abcd••••••••",
				scopes: ["ocr:write"],
				environment: "live",
				last_used_at: null,
				expires_at: null,
				revoked_at: null,
				created_at: "2026-09-13T00:00:00Z",
			},
		];
		api.clearCache();
		fireEvent.click(screen.getByRole("button", { name: /^Refresh$/i }));

		await waitFor(() => {
			expect(screen.queryByText(/Create your first API Key/)).toBeNull();
		});
	});
});
