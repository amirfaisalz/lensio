import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";

describe("App & Dashboard Navigation with Routes", () => {
	beforeEach(() => {
		localStorage.clear();
		window.history.pushState({}, "", "/");
		vi.restoreAllMocks();

		vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
			const url = String(input);
			if (url.includes("/health")) {
				return {
					ok: true,
					json: async () => ({
						status: "ok",
						timestamp: "2026-09-11T00:00:00Z",
					}),
				} as Response;
			}
			if (
				url.includes("/realms/lensio/protocol/openid-connect/token") ||
				url.includes("/api/v1/auth/login")
			) {
				return {
					ok: true,
					status: 200,
					json: async () => ({
						access_token:
							"eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJzdWItZGV2IiwiZW1haWwiOiJkZXZAbGVuc2lvLmRldiIsInByZWZlcnJlZF91c2VybmFtZSI6ImRldiIsIm5hbWUiOiJMZW5zaW8gRGV2ZWxvcGVyIiwicm9sZXMiOlsiZGV2ZWxvcGVyIl19.dev_sig",
						token_type: "Bearer",
						expires_in: 604800,
						user: {
							id: "00000000-0000-0000-0000-000000000001",
							email: "dev@lensio.dev",
							full_name: "Lensio Lead Developer",
							role: "owner",
						},
						organization: {
							id: "00000000-0000-0000-0000-000000000001",
							name: "Default Organization",
							slug: "default",
							plan_code: "free",
						},
					}),
				} as Response;
			}
			if (url.includes("/api/v1/usage/daily")) {
				return { ok: true, json: async () => ({ data: [] }) } as Response;
			}
			if (url.includes("/api/v1/usage/endpoints")) {
				return { ok: true, json: async () => ({ data: [] }) } as Response;
			}
			if (url.includes("/api/v1/usage/records")) {
				return {
					ok: true,
					json: async () => ({ data: [], total: 0, limit: 25, offset: 0 }),
				} as Response;
			}
			if (url.includes("/api/v1/usage")) {
				return {
					ok: true,
					json: async () => ({
						total_requests: 42,
						success_count: 40,
						error_count: 2,
						quota_limit: 1000,
						quota_remaining: 958,
						p95_latency_ms: 180,
						rate_limit_violations: 0,
						billing_cycle_reset: "2026-10-01T00:00:00Z",
					}),
				} as Response;
			}
			if (url.includes("/api/v1/auth/api-keys")) {
				return { ok: true, json: async () => ({ data: [] }) } as Response;
			}
			if (url.includes("/api/v1/account/plan")) {
				return {
					ok: true,
					json: async () => ({
						plan_code: "free",
						plan_name: "Free Tier",
						monthly_quota: 100,
						rate_limit_per_minute: 10,
						billing_cycle_reset: "2026-10-01T00:00:00Z",
					}),
				} as Response;
			}
			if (url.includes("/api/v1/account/members")) {
				return { ok: true, json: async () => ({ data: [] }) } as Response;
			}
			if (url.includes("/api/v1/account")) {
				return {
					ok: true,
					json: async () => ({
						organization_id: "00000000-0000-0000-0000-000000000001",
						organization_name: "Default Organization",
						slug: "default",
						plan_code: "free",
						plan_name: "Free Tier",
						active_keys_count: 1,
						created_at: "2026-09-11T00:00:00Z",
					}),
				} as Response;
			}
			return { ok: true, json: async () => ({}) } as Response;
		});
	});

	it("forcefully redirects /dashboard back to /login when unauthenticated", async () => {
		// Attempt to access protected /dashboard directly while unauthenticated
		window.history.pushState({}, "", "/dashboard");
		render(<App />);

		// Must redirect to /login and display the login form
		await waitFor(() => {
			expect(screen.getByText("Masuk (Sign In)")).toBeDefined();
			expect(window.location.pathname).toBe("/login");
		});

		// 1-click login shortcut must be absent
		expect(screen.queryByText("1-Click Developer Sign-In")).toBeNull();
	});

	it("executes normal login flow and redirects to /dashboard, then navigates across pages", async () => {
		window.history.pushState({}, "", "/login");
		render(<App />);

		// Fill in normal credentials
		fireEvent.change(screen.getByLabelText("Email Kerja / Username"), {
			target: { value: "dev@lensio.dev" },
		});
		fireEvent.change(screen.getByLabelText("Kata Sandi"), {
			target: { value: "dev123" },
		});

		// Submit normal sign in form
		fireEvent.click(screen.getByText("Masuk ke Dashboard"));

		// Must successfully navigate to /dashboard
		await waitFor(() => {
			expect(window.location.pathname).toBe("/dashboard");
			expect(screen.getByText("System Overview")).toBeDefined();
		});

		// Navigate to API Keys
		fireEvent.click(screen.getByText("API Keys"));
		await waitFor(() => {
			expect(window.location.pathname).toBe("/dashboard/keys");
			expect(screen.getByText("API Key Management")).toBeDefined();
		});

		// Navigate to Usage & Analytics
		fireEvent.click(screen.getByText("Usage & Analytics"));
		await waitFor(() => {
			expect(window.location.pathname).toBe("/dashboard/usage");
			expect(screen.getByText("Daily Request Volume")).toBeDefined();
		});

		// Navigate to Requests Explorer
		fireEvent.click(screen.getByText("Requests Explorer"));
		await waitFor(() => {
			expect(window.location.pathname).toBe("/dashboard/requests");
			expect(
				screen.getByText("Requests Explorer", { selector: "h2" }),
			).toBeDefined();
		});

		// Navigate to API Documentation
		fireEvent.click(screen.getByText("API Documentation"));
		await waitFor(() => {
			expect(window.location.pathname).toBe("/dashboard/docs");
			expect(screen.getByText("Quickstart Integration Snippet")).toBeDefined();
		});

		// Navigate to Account & Settings
		fireEvent.click(screen.getByText("Account & Settings"));
		await waitFor(() => {
			expect(window.location.pathname).toBe("/dashboard/account");
			expect(screen.getByText("Organization Profile")).toBeDefined();
		});

		// Test Logout from simplified header
		const logoutBtn = screen.getByLabelText("Keluar dari akun");
		fireEvent.click(logoutBtn);

		// Must redirect back to /login
		await waitFor(() => {
			expect(window.location.pathname).toBe("/login");
			expect(screen.getByText("Masuk (Sign In)")).toBeDefined();
		});
	});

	it("opens and closes mobile navigation drawer via hamburger button and close button", async () => {
		window.history.pushState({}, "", "/login");
		render(<App />);

		// Log in
		fireEvent.change(screen.getByLabelText("Email Kerja / Username"), {
			target: { value: "dev@lensio.dev" },
		});
		fireEvent.change(screen.getByLabelText("Kata Sandi"), {
			target: { value: "dev123" },
		});
		fireEvent.click(screen.getByText("Masuk ke Dashboard"));

		await waitFor(() => {
			expect(window.location.pathname).toBe("/dashboard");
			expect(screen.getByText("System Overview")).toBeDefined();
		});

		// Find mobile hamburger button
		const hamburgerBtn = await screen.findByLabelText("Buka menu navigasi");
		expect(hamburgerBtn).toBeDefined();

		// Initially drawer backdrop should not be present
		expect(screen.queryByLabelText("Tutup menu navigasi")).toBeNull();

		// Click hamburger menu to open drawer
		fireEvent.click(hamburgerBtn);

		// Now backdrop and close button in drawer should be in the document
		expect(screen.getByLabelText("Tutup menu navigasi")).toBeDefined();
		const closeDrawerBtn = screen.getByLabelText("Tutup navigasi");
		expect(closeDrawerBtn).toBeDefined();

		// Close drawer via close button
		fireEvent.click(closeDrawerBtn);
		expect(screen.queryByLabelText("Tutup menu navigasi")).toBeNull();

		// Reopen and close via backdrop
		fireEvent.click(hamburgerBtn);
		expect(screen.getByLabelText("Tutup menu navigasi")).toBeDefined();
		fireEvent.click(screen.getByLabelText("Tutup menu navigasi"));
		expect(screen.queryByLabelText("Tutup menu navigasi")).toBeNull();
	});

	it("switches organizations and creates new organization from bottom-left sidebar dropdown", async () => {
		window.history.pushState({}, "", "/login");
		render(<App />);

		// Log in
		fireEvent.change(screen.getByLabelText("Email Kerja / Username"), {
			target: { value: "dev@lensio.dev" },
		});
		fireEvent.change(screen.getByLabelText("Kata Sandi"), {
			target: { value: "dev123" },
		});
		fireEvent.click(screen.getByText("Masuk ke Dashboard"));

		await waitFor(() => {
			expect(window.location.pathname).toBe("/dashboard");
			expect(screen.getByText("System Overview")).toBeDefined();
		});

		// Find bottom-left organization switcher trigger button
		const orgDropdownBtn = screen.getByLabelText("Ganti organisasi");
		expect(orgDropdownBtn).toBeDefined();
		expect(screen.getByText("Default Organization")).toBeDefined();

		// Click to open dropdown
		fireEvent.click(orgDropdownBtn);

		// Dropdown menu should show organization header and create new org button
		expect(screen.getByText("Organisasi", { selector: "span" })).toBeDefined();
		const createOrgBtn = screen.getByText("Buat Organisasi Baru");
		expect(createOrgBtn).toBeDefined();

		// Click "+ Buat Organisasi Baru" to open modal
		fireEvent.click(createOrgBtn);

		// Modal should open
		expect(screen.getByLabelText("Nama Organisasi / Perusahaan")).toBeDefined();

		// Fill in new organization name
		fireEvent.change(screen.getByLabelText("Nama Organisasi / Perusahaan"), {
			target: { value: "PT Fintek Cemerlang" },
		});

		// Submit creation
		fireEvent.click(screen.getByText("Buat Organisasi & Lanjutkan"));

		// Active organization should now be PT Fintek Cemerlang
		await waitFor(() => {
			expect(screen.getByText("PT Fintek Cemerlang")).toBeDefined();
		});

		// Re-open dropdown to verify multiple organizations in list and switch back
		fireEvent.click(screen.getByLabelText("Ganti organisasi"));
		await waitFor(() => {
			expect(screen.getByText("Default Organization")).toBeDefined();
		});

		// Switch back to Default Organization from dropdown list
		const defaultOrgItems = screen.getAllByText("Default Organization");
		fireEvent.click(defaultOrgItems[defaultOrgItems.length - 1]);

		// Active organization is now switched back
		await waitFor(() => {
			expect(screen.getByText("Default Organization")).toBeDefined();
		});
	});
});

