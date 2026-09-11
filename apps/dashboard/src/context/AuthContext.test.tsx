import { act, render, screen } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it } from "vitest";
import { AuthProvider, useAuth } from "./AuthContext";

const TestConsumer: React.FC = () => {
	const { apiKey, environment, isConnected, setApiKey, logout } = useAuth();
	return (
		<div>
			<span data-testid="key">{apiKey || "none"}</span>
			<span data-testid="env">{environment}</span>
			<span data-testid="status">
				{isConnected ? "connected" : "disconnected"}
			</span>
			<button type="button" onClick={() => setApiKey("nusa_test_123")}>
				Set Test Key
			</button>
			<button type="button" onClick={() => setApiKey("nusa_live_456")}>
				Set Live Key
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

		// Set test key
		act(() => {
			screen.getByText("Set Test Key").click();
		});
		expect(screen.getByTestId("key").textContent).toBe("nusa_test_123");
		expect(screen.getByTestId("env").textContent).toBe("test");
		expect(screen.getByTestId("status").textContent).toBe("connected");

		// Set live key
		act(() => {
			screen.getByText("Set Live Key").click();
		});
		expect(screen.getByTestId("key").textContent).toBe("nusa_live_456");
		expect(screen.getByTestId("env").textContent).toBe("live");

		// Logout
		act(() => {
			screen.getByText("Logout").click();
		});
		expect(screen.getByTestId("key").textContent).toBe("none");
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
