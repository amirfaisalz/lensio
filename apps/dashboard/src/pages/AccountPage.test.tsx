import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../context/AuthContext";
import { AccountPage } from "./AccountPage";

describe("AccountPage", () => {
	beforeEach(() => {
		localStorage.clear();
		vi.restoreAllMocks();
	});

	const mockOrg = {
		organization_id: "org-1234-uuid",
		organization_name: "Acme Corporation",
		slug: "acme-corp",
		plan_code: "free",
		plan_name: "Free Tier",
		active_keys_count: 2,
		created_at: "2026-09-01T00:00:00Z",
	};

	const mockPlan = {
		plan_code: "free",
		plan_name: "Free Tier",
		monthly_quota: 100,
		rate_limit_per_minute: 10,
		billing_cycle_reset: "2026-10-01T00:00:00Z",
	};

	const mockMembers = [
		{
			id: "user-1",
			name: "Admin User",
			email: "admin@acme.test",
			role: "owner",
			created_at: "2026-09-01T00:00:00Z",
		},
	];

	it("renders organization details, current plan, and team members", async () => {
		vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
			const url = String(input);
			if (url.includes("/api/v1/account/plan")) {
				return { ok: true, json: async () => mockPlan } as Response;
			}
			if (url.includes("/api/v1/account/members")) {
				return {
					ok: true,
					json: async () => ({ data: mockMembers }),
				} as Response;
			}
			if (url.includes("/api/v1/account")) {
				return { ok: true, json: async () => mockOrg } as Response;
			}
			return { ok: true, json: async () => ({}) } as Response;
		});

		render(<AccountPage />);

		await waitFor(() => {
			expect(screen.getByText("Acme Corporation")).toBeDefined();
			expect(screen.getByText("acme-corp")).toBeDefined();
			expect(screen.getByText("Current Plan")).toBeDefined();
			expect(screen.getByText("admin@acme.test")).toBeDefined();
			expect(screen.getByText("owner")).toBeDefined();
		});
	});

	it("switches plan to Starter tier", async () => {
		const updatedPlan = {
			...mockPlan,
			plan_code: "starter",
			plan_name: "Starter Tier",
			monthly_quota: 1000,
			rate_limit_per_minute: 30,
		};

		vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
			const url = String(input);
			const method = init?.method || "GET";

			if (url.includes("/api/v1/account/plan") && method === "PUT") {
				return { ok: true, json: async () => updatedPlan } as Response;
			}
			if (url.includes("/api/v1/account/plan")) {
				return { ok: true, json: async () => mockPlan } as Response;
			}
			if (url.includes("/api/v1/account/members")) {
				return {
					ok: true,
					json: async () => ({ data: mockMembers }),
				} as Response;
			}
			if (url.includes("/api/v1/account")) {
				return { ok: true, json: async () => mockOrg } as Response;
			}
			return { ok: true, json: async () => ({}) } as Response;
		});

		render(<AccountPage />);

		await waitFor(() => {
			expect(screen.getByText("Acme Corporation")).toBeDefined();
		});

		// Select Starter plan card
		const starterBtn = screen.getByText("Starter Tier");
		fireEvent.click(starterBtn);

		// "Switch to STARTER" button appears
		await waitFor(() => {
			expect(screen.getByText("Switch to STARTER")).toBeDefined();
		});

		// Submit plan update
		fireEvent.click(screen.getByText("Switch to STARTER"));

		// Success banner appears
		await waitFor(() => {
			expect(screen.getByText("Plan Updated Successfully")).toBeDefined();
		});
	});

	it("displays error banner when loading account info fails", async () => {
		vi.spyOn(globalThis, "fetch").mockResolvedValue({
			ok: false,
			status: 500,
			json: async () => ({
				error: { message: "Account database failure" },
			}),
		} as Response);

		render(<AccountPage />);

		await waitFor(() => {
			expect(screen.getByText("Account database failure")).toBeDefined();
		});
	});

	it("renders placeholder empty state when no organization exists", async () => {
		render(
			<AuthProvider>
				<AccountPage />
			</AuthProvider>,
		);

		await waitFor(() => {
			expect(screen.getByText("Belum Ada Organisasi")).toBeDefined();
			expect(
				screen.getByText(
					"Akun Anda belum terdaftar dalam organisasi mana pun. Buat organisasi baru untuk mulai mengelola profil tenant, memilih paket kuota API, dan mengundang anggota tim.",
				),
			).toBeDefined();
			expect(screen.getByText("Buat Organisasi Sekarang")).toBeDefined();
			expect(screen.getByText("Profil Tenant")).toBeDefined();
			expect(screen.getByText("Paket & Kuota Bulanan")).toBeDefined();
			expect(screen.getByText("Manajemen Anggota")).toBeDefined();
		});
	});

	it("creates an organization from the placeholder modal in AccountPage", async () => {
		vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
			const url = String(input);
			if (url.includes("/api/v1/account/organizations")) {
				return {
					ok: true,
					json: async () => ({
						organization: {
							id: "org-new-id",
							name: "PT Solusi Cerdas",
							slug: "pt-solusi-cerdas",
							plan_code: "free",
						},
					}),
				} as Response;
			}
			if (url.includes("/api/v1/account/plan")) {
				return { ok: true, json: async () => mockPlan } as Response;
			}
			if (url.includes("/api/v1/account/members")) {
				return {
					ok: true,
					json: async () => ({ data: mockMembers }),
				} as Response;
			}
			if (url.includes("/api/v1/account")) {
				return {
					ok: true,
					json: async () => ({
						...mockOrg,
						organization_name: "PT Solusi Cerdas",
						slug: "pt-solusi-cerdas",
					}),
				} as Response;
			}
			return { ok: true, json: async () => ({}) } as Response;
		});

		render(
			<AuthProvider>
				<AccountPage />
			</AuthProvider>,
		);

		// Verify placeholder is shown
		await waitFor(() => {
			expect(screen.getByText("Belum Ada Organisasi")).toBeDefined();
		});

		// Open modal
		fireEvent.click(screen.getByText("Buat Organisasi Sekarang"));
		expect(screen.getByText("Buat Organisasi Baru")).toBeDefined();

		// Fill in org name
		fireEvent.change(screen.getByLabelText("Nama Organisasi / Perusahaan"), {
			target: { value: "PT Solusi Cerdas" },
		});

		// Submit form
		fireEvent.click(screen.getByText("Buat Organisasi & Simpan"));

		// Transitions to active organization account details view
		await waitFor(() => {
			expect(screen.getByText("PT Solusi Cerdas")).toBeDefined();
			expect(screen.getByText("pt-solusi-cerdas")).toBeDefined();
		});
	});
});
