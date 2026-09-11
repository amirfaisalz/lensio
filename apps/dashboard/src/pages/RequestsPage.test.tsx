import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { RequestsPage } from "./RequestsPage";

describe("RequestsPage", () => {
	beforeEach(() => {
		localStorage.clear();
		vi.restoreAllMocks();
	});

	it("renders request logs table with status badges and detail inspector", async () => {
		const mockRecords = [
			{
				id: "rec-001",
				org_id: "org-1",
				api_key_id: "key-1",
				request_id: "req-alpha-12345",
				endpoint: "/api/v1/ocr/ktp",
				status_code: 200,
				latency_ms: 145,
				timestamp: "2026-09-11T12:30:00Z",
			},
			{
				id: "rec-002",
				org_id: "org-1",
				api_key_id: "key-1",
				request_id: "req-beta-67890",
				endpoint: "/api/v1/ocr/ktp",
				status_code: 429,
				latency_ms: 5,
				timestamp: "2026-09-11T12:31:00Z",
			},
		];

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: true,
			json: async () => ({
				data: mockRecords,
				total: 2,
				limit: 25,
				offset: 0,
			}),
		} as Response);

		render(<RequestsPage />);

		await waitFor(() => {
			expect(screen.getAllByText("200 OK").length).toBeGreaterThan(0);
			expect(screen.getAllByText("429 Rate Limited").length).toBeGreaterThan(0);
			expect(screen.getByText("req-alpha-12345")).toBeDefined();
			expect(screen.getByText("145 ms")).toBeDefined();
		});

		// Click "View" button for first record to open inspector modal
		const viewButtons = screen.getAllByTitle("View request details");
		expect(viewButtons.length).toBe(2);
		fireEvent.click(viewButtons[0]);

		// Inspector modal displays details
		expect(screen.getByText("Request Audit Details")).toBeDefined();
		expect(screen.getAllByText("req-alpha-12345").length).toBe(2);

		// Close modal
		fireEvent.click(screen.getByLabelText("Close dialog"));
		await waitFor(() => {
			expect(screen.queryByText("Request Audit Details")).toBeNull();
		});
	});

	it("filters by status code and endpoint search", async () => {
		const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue({
			ok: true,
			json: async () => ({ data: [], total: 0, limit: 25, offset: 0 }),
		} as Response);

		render(<RequestsPage />);

		await waitFor(() => {
			expect(screen.getByText("All Statuses")).toBeDefined();
		});

		// Change status code filter to 429
		const select = screen.getByRole("combobox");
		fireEvent.change(select, { target: { value: "429" } });

		await waitFor(() => {
			expect(fetchSpy).toHaveBeenCalledWith(
				expect.stringContaining("status_code=429"),
				expect.anything(),
			);
		});

		// Search endpoint
		const endpointInput = screen.getByPlaceholderText("e.g. /api/v1/ocr/ktp");
		fireEvent.change(endpointInput, { target: { value: "/api/v1/ocr" } });

		await waitFor(() => {
			expect(fetchSpy).toHaveBeenCalledWith(
				expect.stringContaining("endpoint=%2Fapi%2Fv1%2Focr"),
				expect.anything(),
			);
		});
	});

	it("renders empty state when no request records match", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: true,
			json: async () => ({ data: [], total: 0, limit: 25, offset: 0 }),
		} as Response);

		render(<RequestsPage />);

		await waitFor(() => {
			expect(screen.getByText("No matching requests found.")).toBeDefined();
		});
	});

	it("handles fetch errors gracefully", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: false,
			status: 500,
			json: async () => ({
				error: { message: "Failed to query usage records database" },
			}),
		} as Response);

		render(<RequestsPage />);

		await waitFor(() => {
			expect(
				screen.getByText("Failed to query usage records database"),
			).toBeDefined();
		});
	});
});
