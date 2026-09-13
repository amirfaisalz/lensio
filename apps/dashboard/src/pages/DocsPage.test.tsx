import { fireEvent, render, screen } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../context/AuthContext";
import { DocsPage } from "./DocsPage";

const renderWithAuth = (ui: React.ReactElement) => {
	return render(<AuthProvider>{ui}</AuthProvider>);
};

describe("DocsPage", () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	it("renders documentation sections and standardized error table", () => {
		renderWithAuth(<DocsPage />);

		expect(screen.getByText("API Documentation & Quickstart")).toBeDefined();
		expect(screen.getByText("Quickstart Integration Snippet")).toBeDefined();
		expect(screen.getByText("Standardized API Error Codes")).toBeDefined();
		expect(screen.getByText("Platform Headers")).toBeDefined();

		// Check error codes
		expect(screen.getByText("invalid_api_key")).toBeDefined();
		expect(screen.getByText("rate_limit_exceeded")).toBeDefined();
		expect(screen.getByText("ocr_failed")).toBeDefined();
	});

	it("switches code snippet tabs and copies snippet to clipboard", async () => {
		const writeTextMock = vi.fn().mockResolvedValue(undefined);
		Object.assign(navigator, {
			clipboard: {
				writeText: writeTextMock,
			},
		});

		renderWithAuth(<DocsPage />);

		// Switch to python tab
		const pythonTab = screen.getByRole("button", { name: "python" });
		fireEvent.click(pythonTab);

		const copyButton = screen.getByTitle("Copy snippet");
		fireEvent.click(copyButton);

		expect(writeTextMock).toHaveBeenCalledWith(
			expect.stringContaining("/api/v1/ocr/ktp"),
		);

		// Switch to SIM tab
		const simTab = screen.getByRole("button", { name: /sim/i });
		fireEvent.click(simTab);

		fireEvent.click(copyButton);
		expect(writeTextMock).toHaveBeenCalledWith(
			expect.stringContaining("/api/v1/ocr/sim"),
		);
	});
});
