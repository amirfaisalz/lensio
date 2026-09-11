import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../context/AuthContext";
import { APIKeysPage } from "./APIKeysPage";

const renderWithAuth = (ui: React.ReactElement) => {
	return render(<AuthProvider>{ui}</AuthProvider>);
};

describe("APIKeysPage", () => {
	beforeEach(() => {
		localStorage.clear();
		vi.restoreAllMocks();
	});

	it("renders loading skeleton and then empty state when no keys exist", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValue({
			ok: true,
			json: async () => ({ data: [] }),
		} as Response);

		renderWithAuth(<APIKeysPage />);

		await waitFor(() => {
			expect(screen.getByText("No API Keys Generated")).toBeDefined();
			expect(
				screen.getByText(/Create an API key to start authenticating/),
			).toBeDefined();
		});
	});

	it("renders list of API keys with badges and details", async () => {
		const mockKeys = [
			{
				id: "key-1",
				org_id: "org-1",
				name: "Backend Server Key",
				prefix: "nusa_live_ab12",
				masked_key: "nusa_live_ab12••••••••",
				scopes: ["ocr:read", "ocr:write"],
				environment: "live",
				rate_limit_rpm: 60,
				monthly_quota: 1000,
				is_active: true,
				created_at: "2026-09-10T12:00:00Z",
				last_used_at: "2026-09-11T10:00:00Z",
			},
		];

		vi.spyOn(globalThis, "fetch").mockResolvedValue({
			ok: true,
			json: async () => ({ data: mockKeys }),
		} as Response);

		renderWithAuth(<APIKeysPage />);

		await waitFor(() => {
			expect(screen.getByText("Backend Server Key")).toBeDefined();
			expect(screen.getByText("nusa_live_ab12••••••••")).toBeDefined();
			expect(screen.getByText("ocr:read")).toBeDefined();
			expect(screen.getByText("ocr:write")).toBeDefined();
			expect(screen.getByText("Active")).toBeDefined();
		});
	});

	it("handles error when fetching API keys fails", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValue({
			ok: false,
			status: 500,
			json: async () => ({
				error: { message: "Internal server database error" },
			}),
		} as Response);

		renderWithAuth(<APIKeysPage />);

		await waitFor(() => {
			expect(screen.getByText("Internal server database error")).toBeDefined();
		});
	});

	it("creates a new API key and reveals the plaintext secret token", async () => {
		const createdKeyResponse = {
			id: "key-2",
			org_id: "org-1",
			name: "Client App Key",
			key: "nusa_live_secret_plain_1234567890",
			prefix: "nusa_live_secr",
			scopes: ["ocr:write"],
			environment: "test",
			expires_at: null,
			created_at: "2026-09-11T12:00:00Z",
		};

		const updatedList = [
			{
				id: "key-2",
				org_id: "org-1",
				name: "Client App Key",
				prefix: "nusa_live_secr",
				masked_key: "nusa_live_secr••••••••",
				scopes: ["ocr:write"],
				environment: "test",
				rate_limit_rpm: 60,
				monthly_quota: 1000,
				is_active: true,
				created_at: "2026-09-11T12:00:00Z",
				last_used_at: null,
			},
		];

		let hasCreated = false;

		vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
			const method = init?.method || "GET";
			if (method === "POST") {
				hasCreated = true;
				return { ok: true, json: async () => createdKeyResponse } as Response;
			}
			return {
				ok: true,
				json: async () => ({ data: hasCreated ? updatedList : [] }),
			} as Response;
		});

		renderWithAuth(<APIKeysPage />);

		await waitFor(() => {
			expect(screen.getByText("Create New Key")).toBeDefined();
		});

		// Open Create Modal
		fireEvent.click(screen.getByText("Create New Key"));
		expect(screen.getByText("Create New API Key")).toBeDefined();

		// Fill Name
		const nameInput = screen.getByPlaceholderText(
			"e.g. Production Backend Service",
		);
		fireEvent.change(nameInput, { target: { value: "Client App Key" } });

		// Select Test environment
		fireEvent.click(screen.getByText("Test (Sandbox)"));

		// Submit creation
		const submitBtn = screen.getByRole("button", { name: "Create Key" });
		fireEvent.click(submitBtn);

		// Token reveal modal opens
		await waitFor(() => {
			expect(screen.getByText("Save Your API Key")).toBeDefined();
			expect(
				screen.getByDisplayValue("nusa_live_secret_plain_1234567890"),
			).toBeDefined();
		});

		// Connect in Dashboard saves to local storage and dismisses modal
		fireEvent.click(screen.getByText("Connect in Dashboard"));
		expect(localStorage.getItem("nusaid_api_key")).toBe(
			"nusa_live_secret_plain_1234567890",
		);

		await waitFor(() => {
			expect(screen.queryByText("Save Your API Key")).toBeNull();
		});
	});

	it("revokes an API key after confirmation", async () => {
		const mockKeys = [
			{
				id: "key-to-delete",
				org_id: "org-1",
				name: "Temporary Key",
				prefix: "nusa_live_temp",
				masked_key: "nusa_live_temp••••••••",
				scopes: ["ocr:read"],
				environment: "live",
				rate_limit_rpm: 60,
				monthly_quota: 1000,
				is_active: true,
				created_at: "2026-09-10T12:00:00Z",
				last_used_at: null,
			},
		];

		let isRevoked = false;

		vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
			const method = init?.method || "GET";
			if (method === "DELETE") {
				isRevoked = true;
				return {
					ok: true,
					json: async () => ({
						message: "API key successfully revoked",
						id: "key-to-delete",
					}),
				} as Response;
			}
			return {
				ok: true,
				json: async () => ({ data: isRevoked ? [] : mockKeys }),
			} as Response;
		});

		renderWithAuth(<APIKeysPage />);

		await waitFor(() => {
			expect(screen.getByText("Temporary Key")).toBeDefined();
		});

		// Click Revoke icon button
		const revokeButton = screen.getByTitle("Revoke this API Key");
		fireEvent.click(revokeButton);

		// Confirmation modal
		expect(screen.getByText("Revoke API Key")).toBeDefined();
		expect(
			screen.getByText(/Are you sure you want to revoke this key/),
		).toBeDefined();

		// Confirm Revocation
		fireEvent.click(screen.getByText("Confirm Revocation"));

		await waitFor(() => {
			expect(screen.getByText("No API Keys Generated")).toBeDefined();
		});
	});
});
