import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { UsagePage } from "./UsagePage";

describe("UsagePage", () => {
	beforeEach(() => {
		localStorage.clear();
		vi.restoreAllMocks();
	});

	it("renders daily request volume and endpoint analytics", async () => {
		const mockDaily = [
			{
				date: "2026-09-10",
				total_requests: 120,
				success_count: 118,
				error_count: 2,
				avg_latency_ms: 135,
			},
			{
				date: "2026-09-11",
				total_requests: 250,
				success_count: 245,
				error_count: 5,
				avg_latency_ms: 142,
			},
		];

		const mockEndpoints = [
			{
				endpoint: "/api/v1/ocr/ktp",
				method: "POST",
				total_requests: 350,
				error_count: 6,
				avg_latency_ms: 140,
			},
			{
				endpoint: "/api/v1/usage",
				method: "GET",
				total_requests: 20,
				error_count: 1,
				avg_latency_ms: 25,
			},
		];

		vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
			const url = String(input);
			if (url.includes("/api/v1/usage/daily")) {
				return {
					ok: true,
					json: async () => ({ data: mockDaily }),
				} as Response;
			}
			if (url.includes("/api/v1/usage/endpoints")) {
				return {
					ok: true,
					json: async () => ({ data: mockEndpoints }),
				} as Response;
			}
			return { ok: true, json: async () => ({}) } as Response;
		});

		render(<UsagePage />);

		await waitFor(() => {
			expect(screen.getByText("Usage & Analytics")).toBeDefined();
			expect(screen.getByText("Daily Request Volume")).toBeDefined();
			expect(screen.getByText("Endpoint Breakdown")).toBeDefined();
			expect(screen.getByText("/api/v1/ocr/ktp")).toBeDefined();
			expect(screen.getByText("/api/v1/usage")).toBeDefined();
			expect(screen.getByText("350")).toBeDefined();
			expect(screen.getAllByText(/25/).length).toBeGreaterThan(0);
		});
	});

	it("handles empty usage records gracefully", async () => {
		vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
			const url = String(input);
			if (url.includes("/api/v1/usage/daily")) {
				return { ok: true, json: async () => ({ data: [] }) } as Response;
			}
			if (url.includes("/api/v1/usage/endpoints")) {
				return { ok: true, json: async () => ({ data: [] }) } as Response;
			}
			return { ok: true, json: async () => ({}) } as Response;
		});

		render(<UsagePage />);

		await waitFor(() => {
			expect(
				screen.getByText("No activity recorded in this billing cycle yet."),
			).toBeDefined();
			expect(
				screen.getByText("No endpoint requests recorded yet."),
			).toBeDefined();
		});
	});

	it("displays error banner if fetching usage analytics fails", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValue({
			ok: false,
			status: 500,
			json: async () => ({
				error: { message: "Failed to connect to analytics store" },
			}),
		} as Response);

		render(<UsagePage />);

		await waitFor(() => {
			expect(
				screen.getByText("Failed to connect to analytics store"),
			).toBeDefined();
		});
	});
});
