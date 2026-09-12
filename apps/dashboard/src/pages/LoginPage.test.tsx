import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type React from "react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider, useAuth } from "../context/AuthContext";
import { LoginPage } from "./LoginPage";

const AuthStateDisplay: React.FC = () => {
	const { isConnected, oidcUser, currentOrg, apiKey } = useAuth();
	return (
		<div>
			<span data-testid="auth-status">
				{isConnected ? "authenticated" : "unauthenticated"}
			</span>
			<span data-testid="auth-email">
				{oidcUser?.email || apiKey || "none"}
			</span>
			<span data-testid="auth-org">{currentOrg?.name || "none"}</span>
		</div>
	);
};

const renderLoginPage = (defaultMode: "login" | "register" = "login") => {
	return render(
		<AuthProvider>
			<MemoryRouter>
				<LoginPage defaultMode={defaultMode} />
				<AuthStateDisplay />
			</MemoryRouter>
		</AuthProvider>,
	);
};

describe("LoginPage - Normal Flow & Impeccable Design", () => {
	let verifiedEmails: Set<string>;

	beforeEach(() => {
		localStorage.clear();
		vi.restoreAllMocks();
		verifiedEmails = new Set<string>();

		vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
			const url = String(input);
			let body: Record<string, string> = {};
			if (typeof init?.body === "string") {
				try {
					body = JSON.parse(init.body);
				} catch {
					// fallback
				}
			}

			if (url.includes("/api/v1/auth/register")) {
				return {
					ok: true,
					status: 201,
					json: async () => ({
						status: "pending_verification",
						email: body.email,
						verification_token: "tok-test",
						message: "Registrasi berhasil.",
					}),
				} as Response;
			}

			if (url.includes("/api/v1/auth/verify-email")) {
				if (body.email) {
					verifiedEmails.add(body.email);
				}
				return {
					ok: true,
					status: 200,
					json: async () => ({
						status: "verified",
						message: "Email berhasil diverifikasi!",
					}),
				} as Response;
			}

			if (url.includes("/api/v1/auth/login")) {
				if (
					!verifiedEmails.has(body.email) &&
					body.email !== "dev@lensio.dev"
				) {
					return {
						ok: false,
						status: 403,
						json: async () => ({
							error: {
								code: "EMAIL_NOT_VERIFIED",
								message:
									"Akun Anda belum diverifikasi. Silakan periksa email Anda dan lakukan verifikasi sebelum masuk.",
							},
						}),
					} as Response;
				}

				return {
					ok: true,
					status: 200,
					json: async () => ({
						status: "authenticated",
						user: {
							id: "u-rizky",
							email: body.email,
							full_name: "Rizky Ramadhan",
							role: "member",
						},
						organization: null,
					}),
				} as Response;
			}

			return { ok: true, json: async () => ({}) } as Response;
		});
	});

	it("strictly removes 1-click developer sign-in buttons and renders clean auth portal", () => {
		renderLoginPage();

		// Verifies brand header elements
		expect(screen.getByText("Lensio Developer Portal")).toBeDefined();

		// Strictly verifies 1-click shortcuts are NOT rendered anywhere
		expect(screen.queryByText("1-Click Developer Sign-In")).toBeNull();
		expect(screen.queryByText("dev@lensio.dev")).toBeNull();
		expect(screen.queryByText("admin@lensio.dev")).toBeNull();

		// Initial state is unauthenticated
		expect(screen.getByTestId("auth-status").textContent).toBe(
			"unauthenticated",
		);
	});

	it("validates empty email and empty password upon sign-in submission", () => {
		renderLoginPage();

		const submitBtn = screen.getByText("Masuk ke Dashboard");
		fireEvent.click(submitBtn);

		// Expect validation error for empty email
		expect(
			screen.getByText("Silakan masukkan email kerja atau username Anda."),
		).toBeDefined();

		// Provide email, submit again without password
		fireEvent.change(screen.getByLabelText("Email Kerja / Username"), {
			target: { value: "user@perusahaan.id" },
		});
		fireEvent.click(submitBtn);

		// Expect validation error for empty password
		expect(screen.getByText("Silakan masukkan kata sandi Anda.")).toBeDefined();
	});

	it("rejects login with invalid credentials and displays clear error message (no bypass)", async () => {
		// Mock fetch to simulate 401 Unauthorized from Keycloak
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
			ok: false,
			status: 401,
			json: async () => ({ error: "invalid_grant" }),
		} as Response);

		renderLoginPage();

		fireEvent.change(screen.getByLabelText("Email Kerja / Username"), {
			target: { value: "developer@perusahaan.id" },
		});
		fireEvent.change(screen.getByLabelText("Kata Sandi"), {
			target: { value: "wrong_password" },
		});

		fireEvent.click(screen.getByText("Masuk ke Dashboard"));

		await waitFor(() => {
			expect(
				screen.getByText(
					"Email atau kata sandi tidak valid. Silakan periksa kembali kredensial Anda.",
				),
			).toBeDefined();
			expect(screen.getByTestId("auth-status").textContent).toBe(
				"unauthenticated",
			);
		});
	});

	it("toggles password visibility with accessible eye button", () => {
		renderLoginPage();

		const passwordInput = screen.getByLabelText(
			"Kata Sandi",
		) as HTMLInputElement;
		expect(passwordInput.type).toBe("password");

		const toggleBtn = screen.getByLabelText("Tampilkan kata sandi");
		fireEvent.click(toggleBtn);

		expect(passwordInput.type).toBe("text");
		expect(screen.getByLabelText("Sembunyikan kata sandi")).toBeDefined();

		fireEvent.click(screen.getByLabelText("Sembunyikan kata sandi"));
		expect(passwordInput.type).toBe("password");
	});

	it("enforces registration validation (name, email, password length, match, terms)", () => {
		renderLoginPage("register");

		const submitBtn = screen.getByText("Daftar Sekarang");

		// 1. Empty name
		fireEvent.click(submitBtn);
		expect(
			screen.getByText("Silakan masukkan nama lengkap Anda."),
		).toBeDefined();

		// 2. Invalid email
		fireEvent.change(screen.getByLabelText("Nama Lengkap"), {
			target: { value: "Budi Santoso" },
		});
		fireEvent.click(submitBtn);
		expect(
			screen.getByText("Silakan masukkan alamat email kerja yang valid."),
		).toBeDefined();

		// 3. Short password
		fireEvent.change(screen.getByLabelText("Email Kerja"), {
			target: { value: "budi@fintech.id" },
		});
		fireEvent.change(screen.getByLabelText("Kata Sandi"), {
			target: { value: "123" },
		});
		fireEvent.click(submitBtn);
		expect(
			screen.getByText("Kata sandi harus memiliki panjang minimal 8 karakter."),
		).toBeDefined();

		// 4. Password mismatch
		fireEvent.change(screen.getByLabelText("Kata Sandi"), {
			target: { value: "password123" },
		});
		fireEvent.change(screen.getByLabelText("Konfirmasi Kata Sandi"), {
			target: { value: "different123" },
		});
		fireEvent.click(submitBtn);
		expect(
			screen.getByText("Konfirmasi kata sandi tidak cocok dengan kata sandi."),
		).toBeDefined();

		// 5. Terms not accepted
		fireEvent.change(screen.getByLabelText("Konfirmasi Kata Sandi"), {
			target: { value: "password123" },
		});
		fireEvent.click(submitBtn);
		expect(
			screen.getByText(
				"Anda harus menyetujui Ketentuan Layanan & Kebijakan Privasi.",
			),
		).toBeDefined();
	});

	it("successfully registers user, shows email verification notice, and blocks login until verified", async () => {
		renderLoginPage("register");

		// Fill in registration form (user identity only, no workspace)
		fireEvent.change(screen.getByLabelText("Nama Lengkap"), {
			target: { value: "Rizky Ramadhan" },
		});
		fireEvent.change(screen.getByLabelText("Email Kerja"), {
			target: { value: "rizky@fintech.id" },
		});
		fireEvent.change(screen.getByLabelText("Kata Sandi"), {
			target: { value: "SecretPass123!" },
		});
		fireEvent.change(screen.getByLabelText("Konfirmasi Kata Sandi"), {
			target: { value: "SecretPass123!" },
		});
		fireEvent.click(screen.getByRole("checkbox"));

		// Submit registration
		fireEvent.click(screen.getByText("Daftar Sekarang"));

		// Must transition to Email Verification Pending notice screen
		await waitFor(() => {
			expect(screen.getByText("Verifikasi Email Anda")).toBeDefined();
			expect(screen.getByText("rizky@fintech.id")).toBeDefined();
		});

		// Go back to login without verifying
		fireEvent.click(screen.getByText("Kembali ke Halaman Masuk"));

		// Attempt login with unverified email
		fireEvent.change(screen.getByLabelText("Email Kerja / Username"), {
			target: { value: "rizky@fintech.id" },
		});
		fireEvent.change(screen.getByLabelText("Kata Sandi"), {
			target: { value: "SecretPass123!" },
		});
		fireEvent.click(screen.getByText("Masuk ke Dashboard"));

		// Login must be blocked because user is not verified
		await waitFor(() => {
			expect(
				screen.getByText(
					"Akun Anda belum diverifikasi. Silakan periksa email Anda dan lakukan verifikasi sebelum masuk.",
				),
			).toBeDefined();
			expect(screen.getByTestId("auth-status").textContent).toBe(
				"unauthenticated",
			);
		});

		// Click simulated verification link
		fireEvent.click(screen.getByText("Lakukan verifikasi email sekarang"));

		// Confirmation toast is displayed
		await waitFor(() => {
			expect(
				screen.getByText(
					"Email rizky@fintech.id berhasil diverifikasi! Silakan masuk dengan kata sandi Anda.",
				),
			).toBeDefined();
		});

		// Now log in with verified account
		fireEvent.change(screen.getByLabelText("Kata Sandi"), {
			target: { value: "SecretPass123!" },
		});
		fireEvent.click(screen.getByText("Masuk ke Dashboard"));

		await waitFor(() => {
			expect(screen.getByTestId("auth-status").textContent).toBe(
				"authenticated",
			);
			expect(screen.getByTestId("auth-email").textContent).toBe(
				"rizky@fintech.id",
			);
			// Fresh login starts with NO organization (currentOrg is null)
			expect(screen.getByTestId("auth-org").textContent).toBe("none");
		});
	});

	it("renders pure credential login without API key token input", () => {
		renderLoginPage();

		// Only Email and Password fields are present
		expect(screen.getByLabelText("Email Kerja / Username")).toBeDefined();
		expect(screen.getByLabelText("Kata Sandi")).toBeDefined();
		expect(screen.queryByText("API Key Token")).toBeNull();
		expect(screen.queryByLabelText("Lensio API Key")).toBeNull();
	});
});
