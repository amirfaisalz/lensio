import { fireEvent, render, screen } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it } from "vitest";
import { ThemeProvider, useTheme } from "./ThemeContext";

const ThemeProbe: React.FC = () => {
	const { theme, toggleTheme } = useTheme();
	return (
		<div>
			<span data-testid="theme-value">{theme}</span>
			<button type="button" onClick={toggleTheme}>
				Toggle Theme
			</button>
		</div>
	);
};

describe("ThemeContext", () => {
	beforeEach(() => {
		localStorage.clear();
		document.documentElement.classList.remove("dark");
	});

	it("defaults to light and does not set the dark class", () => {
		render(
			<ThemeProvider>
				<ThemeProbe />
			</ThemeProvider>,
		);
		expect(screen.getByTestId("theme-value").textContent).toBe("light");
		expect(document.documentElement.classList.contains("dark")).toBe(false);
	});

	it("toggles to dark, sets the dark class, and persists the choice", () => {
		render(
			<ThemeProvider>
				<ThemeProbe />
			</ThemeProvider>,
		);
		fireEvent.click(screen.getByText("Toggle Theme"));
		expect(screen.getByTestId("theme-value").textContent).toBe("dark");
		expect(document.documentElement.classList.contains("dark")).toBe(true);
		expect(localStorage.getItem("lensio_theme")).toBe("dark");

		fireEvent.click(screen.getByText("Toggle Theme"));
		expect(screen.getByTestId("theme-value").textContent).toBe("light");
		expect(document.documentElement.classList.contains("dark")).toBe(false);
		expect(localStorage.getItem("lensio_theme")).toBe("light");
	});

	it("restores the persisted theme on mount", () => {
		localStorage.setItem("lensio_theme", "dark");
		render(
			<ThemeProvider>
				<ThemeProbe />
			</ThemeProvider>,
		);
		expect(screen.getByTestId("theme-value").textContent).toBe("dark");
		expect(document.documentElement.classList.contains("dark")).toBe(true);
	});
});
