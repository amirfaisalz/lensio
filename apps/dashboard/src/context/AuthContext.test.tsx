import { act, render, screen, waitFor } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "../services/api";
import { AuthProvider, useAuth } from "./AuthContext";

const TestConsumer: React.FC = () => {
	const {
		apiKey,
		environment,
		isConnected,
		authMode,
		oidcUser,
		currentOrg,
		setApiKey,
		loginOIDC,
		registerOrLogin,
		switchOrganization,
		switchAuthMode,
		createOrganization,
		registerUser,
		verifyUserEmail,
		logout,
	} = useAuth();
	return (
		<div>
			<span data-testid="key">{apiKey || "none"}</span>
			<span data-testid="env">{environment}</span>
			<span data-testid="status">
				{isConnected ? "connected" : "disconnected"}
			</span>
			<span data-testid="mode">{authMode}</span>
			<span data-testid="oidc-email">{oidcUser?.email || "none"}</span>
			<span data-testid="org-name">{currentOrg?.name || "none"}</span>
			<button type="button" onClick={() => setApiKey("lensio_test_123")}>
				Set Test Key
			</button>
			<button type="button" onClick={() => setApiKey("lensio_live_456")}>
				Set Live Key
			</button>
			<button
				type="button"
				onClick={() =>
					loginOIDC({
						sub: "sub-admin-1",
						email: "admin@lensio.dev",
						preferredUsername: "admin",
						roles: ["admin"],
						token: "eyJhbGciOiJSUzI1NiJ9.test.sig",
					})
				}
			>
				Login OIDC
			</button>
			<button
				type="button"
				onClick={() =>
					registerOrLogin(
						"founder@startup.com",
						"Startup Founder",
						"Startup Inc",
						"owner",
					)
				}
			>
				Register Startup
			</button>
			<button
				type="button"
				onClick={() =>
					switchOrganization({
						id: "org-new",
						name: "New Enterprise",
						slug: "new-enterprise",
						planCode: "pro",
					})
				}
			>
				Switch New Org
			</button>
			<button type="button" onClick={() => switchAuthMode("apikey")}>
				Switch API Key
			</button>
			<button type="button" onClick={() => switchAuthMode("oidc")}>
				Switch OIDC
			</button>
			<button
				type="button"
				onClick={() => createOrganization("Fintech Asia", "starter")}
			>
				Create Dynamic Org
			</button>
			<button
				type="button"
				onClick={() => {
					void registerUser("Ali", "ali@test.id", "pass1234").catch(() => {});
				}}
			>
				Register Ali
			</button>
			<button
				type="button"
				onClick={() => {
					void verifyUserEmail("ali@test.id").catch(() => {});
				}}
			>
				Verify Ali
			</button>
			<button type="button" onClick={logout}>
				Logout
			</button>
		</div>
	);
};

describe("AuthContext", () => {
	beforeEach(() => {
		localStorage.clear();
		vi.restoreAllMocks();
	});

	it("provides default state and updates environment automatically", () => {
		render(
			<AuthProvider>
				<TestConsumer />
			</AuthProvider>,
		);

		expect(screen.getByTestId("key").textContent).toBe("none");
		expect(screen.getByTestId("status").textContent).toBe("disconnected");
		expect(screen.getByTestId("mode").textContent).toBe("apikey");
		expect(screen.getByTestId("oidc-email").textContent).toBe("none");

		// Set test key
		act(() => {
			screen.getByText("Set Test Key").click();
		});
		expect(screen.getByTestId("key").textContent).toBe("lensio_test_123");
		expect(screen.getByTestId("env").textContent).toBe("test");
		expect(screen.getByTestId("status").textContent).toBe("connected");

		// Set live key
		act(() => {
			screen.getByText("Set Live Key").click();
		});
		expect(screen.getByTestId("key").textContent).toBe("lensio_live_456");
		expect(screen.getByTestId("env").textContent).toBe("live");

		// Logout
		act(() => {
			screen.getByText("Logout").click();
		});
		expect(screen.getByTestId("key").textContent).toBe("none");
		expect(screen.getByTestId("status").textContent).toBe("disconnected");
	});

	it("supports OIDC session login and mode switching", () => {
		render(
			<AuthProvider>
				<TestConsumer />
			</AuthProvider>,
		);

		// Login with OIDC
		act(() => {
			screen.getByText("Login OIDC").click();
		});
		expect(screen.getByTestId("mode").textContent).toBe("oidc");
		expect(screen.getByTestId("oidc-email").textContent).toBe(
			"admin@lensio.dev",
		);
		expect(screen.getByTestId("status").textContent).toBe("connected");

		// Set an API Key, which automatically sets mode to apikey
		act(() => {
			screen.getByText("Set Live Key").click();
		});
		expect(screen.getByTestId("mode").textContent).toBe("apikey");
		expect(screen.getByTestId("key").textContent).toBe("lensio_live_456");

		// Switch back to OIDC mode
		act(() => {
			screen.getByText("Switch OIDC").click();
		});
		expect(screen.getByTestId("mode").textContent).toBe("oidc");
		expect(screen.getByTestId("oidc-email").textContent).toBe(
			"admin@lensio.dev",
		);

		// Logout in OIDC mode
		act(() => {
			screen.getByText("Logout").click();
		});
		expect(screen.getByTestId("oidc-email").textContent).toBe("none");
		expect(screen.getByTestId("status").textContent).toBe("disconnected");
	});

	it("supports registering user and establishing organization context", () => {
		render(
			<AuthProvider>
				<TestConsumer />
			</AuthProvider>,
		);

		// Register new user and workspace
		act(() => {
			screen.getByText("Register Startup").click();
		});

		expect(screen.getByTestId("status").textContent).toBe("connected");
		expect(screen.getByTestId("mode").textContent).toBe("oidc");
		expect(screen.getByTestId("oidc-email").textContent).toBe(
			"founder@startup.com",
		);
		expect(screen.getByTestId("org-name").textContent).toBe("Startup Inc");

		// Switch organization
		act(() => {
			screen.getByText("Switch New Org").click();
		});
		expect(screen.getByTestId("org-name").textContent).toBe("New Enterprise");

		// Logout clears organization
		act(() => {
			screen.getByText("Logout").click();
		});
		expect(screen.getByTestId("org-name").textContent).toBe("none");
		expect(screen.getByTestId("status").textContent).toBe("disconnected");
	});

	it("supports dynamic organization creation and user email verification lifecycle", async () => {
		render(
			<AuthProvider>
				<TestConsumer />
			</AuthProvider>,
		);

		// Initial org is none
		expect(screen.getByTestId("org-name").textContent).toBe("none");

		// Create dynamic organization
		await act(async () => {
			screen.getByText("Create Dynamic Org").click();
		});

		expect(screen.getByTestId("org-name").textContent).toBe("Fintech Asia");

		// Register user
		await act(async () => {
			screen.getByText("Register Ali").click();
		});

		// Verify user
		await act(async () => {
			screen.getByText("Verify Ali").click();
		});

		// Logout resets organization
		act(() => {
			screen.getByText("Logout").click();
		});
		expect(screen.getByTestId("org-name").textContent).toBe("none");
	});

	it("restores user session and organization from api.fetchCurrentUser on mount in oidc mode", async () => {
		localStorage.setItem("lensio_auth_mode", "oidc");
		vi.spyOn(api, "fetchCurrentUser").mockResolvedValue({
			user: {
				id: "u-session-1",
				email: "session@lensio.dev",
				full_name: "Session User",
				roles: ["developer"],
			},
			organization: {
				id: "org-session-1",
				name: "Restored Org",
				slug: "restored-org",
				plan_code: "starter",
			},
		});

		render(
			<AuthProvider>
				<TestConsumer />
			</AuthProvider>,
		);

		await waitFor(() => {
			expect(screen.getByTestId("mode").textContent).toBe("oidc");
			expect(screen.getByTestId("oidc-email").textContent).toBe(
				"session@lensio.dev",
			);
			expect(screen.getByTestId("org-name").textContent).toBe("Restored Org");
		});
	});

	it("clears user session when api.fetchCurrentUser fails on mount in oidc mode", async () => {
		localStorage.setItem("lensio_auth_mode", "oidc");
		vi.spyOn(api, "fetchCurrentUser").mockRejectedValue(
			new Error("Session expired or unauthorized"),
		);

		render(
			<AuthProvider>
				<TestConsumer />
			</AuthProvider>,
		);

		await waitFor(() => {
			expect(screen.getByTestId("oidc-email").textContent).toBe("none");
			expect(screen.getByTestId("org-name").textContent).toBe("none");
		});
	});

	it("throws error when useAuth is called outside AuthProvider", () => {
		const BadConsumer: React.FC = () => {
			useAuth();
			return null;
		};

		expect(() => render(<BadConsumer />)).toThrow(
			"useAuth must be used within an AuthProvider",
		);
	});
});
