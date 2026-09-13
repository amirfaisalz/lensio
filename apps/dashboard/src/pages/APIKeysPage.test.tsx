import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../context/AuthContext";
import { api } from "../services/api";
import { APIKeysPage } from "./APIKeysPage";

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

describe("APIKeysPage", () => {
	beforeEach(() => {
		localStorage.clear();
		api.clearCache();
		vi.restoreAllMocks();
	});

	it("renders empty state requiring organization when currentOrg is null", () => {
		renderWithAuth(<APIKeysPage />);

		expect(screen.getByText("Organisasi Diperlukan")).toBeDefined();
		expect(
			screen.getByText(
				"Anda harus membuat atau memilih organisasi terlebih dahulu sebelum dapat membuat API Key. Setiap API Key terikat pada kuota bulanan dan kebijakan keamanan organisasi Anda.",
			),
		).toBeDefined();
		expect(screen.getByText("Buat Organisasi Sekarang")).toBeDefined();
	});

	it("creates an organization from the empty state modal", async () => {
		renderWithAuth(<APIKeysPage />);

		// Open org modal from empty state
		fireEvent.click(screen.getByText("Buat Organisasi Sekarang"));
		expect(screen.getByText("Buat Organisasi Baru")).toBeDefined();

		// Fill in org name
		fireEvent.change(screen.getByLabelText("Nama Organisasi / Perusahaan"), {
			target: { value: "PT Fintech Prima" },
		});

		// Submit org creation
		fireEvent.click(screen.getByText("Buat Organisasi & Lanjutkan"));

		// Now the empty state transitions to the active keys management view
		await waitFor(() => {
			expect(screen.getByText("Create New Key")).toBeDefined();
			expect(screen.getByText("Belum Ada API Key")).toBeDefined();
		});
	});

	it("renders empty keys state when organization exists but no keys are generated", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValue({
			ok: true,
			json: async () => ({ data: [] }),
		} as Response);

		renderWithAuth(<APIKeysPage />, { initialOrg: TEST_ORG });

		await waitFor(() => {
			expect(screen.getByText("Belum Ada API Key")).toBeDefined();
			expect(screen.getByText("Buat API Key Pertama")).toBeDefined();
		});
	});

	it("renders list of API keys with badges and details", async () => {
		const mockKeys = [
			{
				id: "key-1",
				org_id: "org-1",
				name: "Backend Server Key",
				prefix: "lensio_live_ab12",
				masked_key: "lensio_live_ab12••••••••",
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

		renderWithAuth(<APIKeysPage />, { initialOrg: TEST_ORG });

		await waitFor(() => {
			expect(screen.getByText("Backend Server Key")).toBeDefined();
			expect(screen.getByText("lensio_live_ab12••••••••")).toBeDefined();
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

		renderWithAuth(<APIKeysPage />, { initialOrg: TEST_ORG });

		await waitFor(() => {
			expect(screen.getByText("Internal server database error")).toBeDefined();
		});
	});

	it("creates a new API key and reveals the plaintext secret token", async () => {
		const createdKeyResponse = {
			id: "key-2",
			org_id: "org-1",
			name: "Client App Key",
			key: "lensio_live_secret_plain_1234567890",
			prefix: "lensio_live_secr",
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
				prefix: "lensio_live_secr",
				masked_key: "lensio_live_secr••••••••",
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
				return {
					ok: true,
					json: async () => createdKeyResponse,
				} as Response;
			}
			return {
				ok: true,
				json: async () => ({ data: hasCreated ? updatedList : [] }),
			} as Response;
		});

		renderWithAuth(<APIKeysPage />, { initialOrg: TEST_ORG });

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
				screen.getByDisplayValue("lensio_live_secret_plain_1234567890"),
			).toBeDefined();
		});

		// Test Copy button
		const copyBtn = screen.getByText("Copy");
		fireEvent.click(copyBtn);
		expect(screen.getByText("Copied")).toBeDefined();

		// Close reveal modal using "I Have Saved It"
		fireEvent.click(screen.getByText("I Have Saved It"));
		expect(screen.queryByText("Save Your API Key")).toBeNull();
	});

	it("revokes an active API key after confirmation", async () => {
		const activeKey = {
			id: "key-to-delete",
			org_id: "org-1",
			name: "Temporary Key",
			prefix: "lensio_live_temp",
			masked_key: "lensio_live_temp••••••••",
			scopes: ["ocr:read"],
			environment: "live",
			rate_limit_rpm: 60,
			monthly_quota: 1000,
			is_active: true,
			created_at: "2026-09-10T12:00:00Z",
			last_used_at: null,
		};

		let isRevoked = false;

		vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
			const method = init?.method || "GET";
			if (method === "DELETE") {
				isRevoked = true;
				return {
					ok: true,
					json: async () => ({
						message: "Key revoked successfully",
						id: "key-to-delete",
					}),
				} as Response;
			}
			return {
				ok: true,
				json: async () => ({
					data: isRevoked
						? [{ ...activeKey, revoked_at: "2026-09-11T14:00:00Z" }]
						: [activeKey],
				}),
			} as Response;
		});

		renderWithAuth(<APIKeysPage />, { initialOrg: TEST_ORG });

		await waitFor(() => {
			expect(screen.getByText("Temporary Key")).toBeDefined();
		});

		// Click revoke trash icon
		const revokeButton = screen.getByTitle("Revoke this API Key");
		fireEvent.click(revokeButton);

		// Modal should open
		expect(
			screen.getByText("Are you sure you want to revoke this key?"),
		).toBeDefined();

		// Confirm revocation
		const confirmBtn = screen.getByText("Confirm Revocation");
		fireEvent.click(confirmBtn);

		await waitFor(() => {
			expect(screen.getByText("Revoked")).toBeDefined();
		});
	});
});
