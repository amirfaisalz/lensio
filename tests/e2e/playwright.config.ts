import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig, devices } from "@playwright/test";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export default defineConfig({
	testDir: __dirname,
	testMatch: "**/*.spec.ts",
	testIgnore: ["**/apps/**", "**/node_modules/**"],
	timeout: 45 * 1000,
	expect: {
		timeout: 10 * 1000,
	},
	fullyParallel: false,
	retries: 0,
	workers: 1,
	reporter: [["list"]],
	use: {
		baseURL: process.env.DASHBOARD_URL || "http://localhost:3000",
		trace: "on-first-retry",
		headless: true,
	},
	projects: [
		{
			name: "chromium",
			use: {
				...devices["Desktop Chrome"],
				...(process.env.PLAYWRIGHT_CHANNEL
					? { channel: process.env.PLAYWRIGHT_CHANNEL }
					: process.env.CI
						? {}
						: { channel: "chrome" }),
			},
		},
	],
});
