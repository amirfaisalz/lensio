import { act, render, screen } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it } from "vitest";
import { AuthProvider, useAuth } from "./AuthContext";

const TestConsumer: React.FC = () => {
	const {
		apiKey,
		environment,
		isConnected,
		authMode,
		oidcUser,
		setApiKey,
		loginOIDC,
		switchAuthMode,
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
			<button type="button" onClick={() => switchAuthMode("apikey")}>
				Switch API Key
			</button>
			<button type="button" onClick={() => switchAuthMode("oidc")}>
				Switch OIDC
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
