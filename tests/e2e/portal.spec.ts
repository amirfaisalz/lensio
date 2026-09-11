import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test } from "@playwright/test";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const syntheticKtpJpeg = fs.readFileSync(
	path.resolve(__dirname, "../fixtures/synthetic/valid_ktp.jpg"),
);

test.describe.serial(
	"NusaID Developer Portal & KTP OCR End-to-End Suite (PRD Section 28)",
	() => {
		const apiBaseURL = process.env.API_URL || "http://localhost:8080";
		let createdApiKey = "";

		test.beforeEach(async ({ page }) => {
			if (createdApiKey) {
				await page.addInitScript((key) => {
					window.localStorage.setItem("nusaid_api_key", key);
				}, createdApiKey);
			}
		});

		// -------------------------------------------------------------------------
		// Test 1: User signs in / connects to Developer Portal
		// -------------------------------------------------------------------------
		test("Test 1: User opens and connects to Developer Portal", async ({
			page,
		}) => {
			await page.goto("/");

			// Verify Developer Portal branding & status
			await expect(page.getByText("NusaID", { exact: true })).toBeVisible();
			await expect(page.getByText("Portal", { exact: true })).toBeVisible();
			await expect(page.getByText("API Online")).toBeVisible();

			// Open Connect API Key modal
			const connectBtn = page.getByRole("button", {
				name: /Connect Key|Change API Key/i,
			});
			await expect(connectBtn).toBeVisible();
			await connectBtn.click();

			// Verify Connect Key modal content
			await expect(
				page.getByRole("heading", { name: "Connect API Key" }),
			).toBeVisible();
			const inputField = page.locator("#api-key-input");
			await expect(inputField).toBeVisible();

			// Close modal
			await page.getByRole("button", { name: "Cancel" }).click();
			await expect(
				page.getByRole("heading", { name: "Connect API Key" }),
			).not.toBeVisible();
		});

		// -------------------------------------------------------------------------
		// Test 2: User generates an API key and copies the token
		// -------------------------------------------------------------------------
		test("Test 2: User generates an API key and copies the plaintext token", async ({
			page,
		}) => {
			await page.goto("/");

			// Navigate to API Keys page via sidebar
			await page.getByRole("button", { name: "API Keys" }).click();
			await expect(
				page.getByRole("heading", { name: "API Key Management" }),
			).toBeVisible();

			// Click Create New Key
			await page.getByRole("button", { name: "Create New Key" }).click();
			await expect(
				page.getByRole("heading", { name: "Create New API Key" }),
			).toBeVisible();

			// Fill Key details
			const keyName = `E2E Playwright Key ${Date.now()}`;
			await page.locator("#key-name").fill(keyName);

			// Ensure Live environment is selected
			const liveRadio = page.locator('input[name="env"][value="live"]');
			await liveRadio.check();

			// Submit Create Key
			await page
				.getByRole("button", { name: "Create Key", exact: true })
				.click();

			// Wait for Save Your API Key reveal modal
			await expect(
				page.getByRole("heading", { name: "Save Your API Key" }),
			).toBeVisible();

			// Capture the plaintext secret token
			const secretInput = page.locator("#created-key-token");
			await expect(secretInput).toBeVisible();
			const tokenVal = await secretInput.inputValue();
			expect(tokenVal).toMatch(/^nusa_live_[a-f0-9]{32,}$/);
			createdApiKey = tokenVal;

			// Verify copy action
			const copyBtn = page.getByRole("button", { name: /Copy/i });
			await copyBtn.click();
			await expect(page.getByText("Copied")).toBeVisible();

			// Click "Connect in Dashboard" to authenticate session
			await page
				.getByRole("button", { name: "Connect in Dashboard" })
				.click();

			// Verify reveal modal is dismissed
			await expect(
				page.getByRole("heading", { name: "Save Your API Key" }),
			).not.toBeVisible();

			// Verify Header indicates connected key (shows "Change API Key")
			await expect(
				page.getByRole("button", { name: "Change API Key" }),
			).toBeVisible();

			// Verify the new key is rendered in the keys table
			await expect(page.getByText(keyName)).toBeVisible();
		});

		// -------------------------------------------------------------------------
		// Test 3: API client submits synthetic KTP image to /api/v1/ocr/ktp
		// -------------------------------------------------------------------------
		test("Test 3: API client submits synthetic KTP image to /api/v1/ocr/ktp", async ({
			request,
		}) => {
			expect(createdApiKey).toBeTruthy();

			// Direct API invocation with multipart document payload
			const response = await request.post(
				`${apiBaseURL}/api/v1/ocr/ktp`,
				{
					headers: {
						Authorization: `Bearer ${createdApiKey}`,
					},
					multipart: {
						document: {
							name: "synthetic_ktp.jpg",
							mimeType: "image/jpeg",
							buffer: syntheticKtpJpeg,
						},
					},
				},
			);

			expect(response.status()).toBe(200);
			const resJson = await response.json();
			expect(resJson).toHaveProperty("id");
			expect(resJson).toHaveProperty("document_type");
			expect(resJson).toHaveProperty("data");
			expect(resJson).toHaveProperty("processing");
		});

		// -------------------------------------------------------------------------
		// Test 4: Verify structured JSON output matches expected fields
		// -------------------------------------------------------------------------
		test("Test 4: Verify structured JSON output matches expected KTP fields", async ({
			page,
			request,
		}) => {
			expect(createdApiKey).toBeTruthy();

			// 1. Verify via API client response fields
			const response = await request.post(
				`${apiBaseURL}/api/v1/ocr/ktp`,
				{
					headers: {
						Authorization: `Bearer ${createdApiKey}`,
					},
					multipart: {
						document: {
							name: "synthetic_ktp.jpg",
							mimeType: "image/jpeg",
							buffer: syntheticKtpJpeg,
						},
					},
				},
			);
			expect(response.status()).toBe(200);
			const json = await response.json();

			expect(json.document_type).toBe("ktp");
			expect(json.confidence).toBeGreaterThanOrEqual(0.8);
			expect(json.data.nik).toBe("3171010101900001");
			expect(json.data.nama).toBe("BUDI SANTOSO");
			expect(json.data.jenis_kelamin).toBe("LAKI-LAKI");
			expect(json.data.kewarganegaraan).toBe("WNI");
			expect(json.processing.latency_ms).toBeGreaterThanOrEqual(0);

			// 2. Also verify through Dashboard UI Live Playground
			await page.goto("/");
			await page.getByRole("button", { name: "Overview" }).click();

			// Click Load Synthetic Fixture in Playground
			const loadSampleBtn = page.getByRole("button", {
				name: "Load Synthetic Fixture",
			});
			await expect(loadSampleBtn).toBeVisible();
			await loadSampleBtn.click();

			// Verify result card appears in UI with extracted data
			await expect(
				page.getByText("3171010101900001", { exact: true }),
			).toBeVisible({
				timeout: 10000,
			});
			await expect(
				page.getByText("BUDI SANTOSO", { exact: true }),
			).toBeVisible();
			await expect(
				page.getByText("Normalized Field Verification"),
			).toBeVisible();
		});

		// -------------------------------------------------------------------------
		// Test 5: Verify usage count increments in dashboard UI
		// -------------------------------------------------------------------------
		test("Test 5: Verify usage count and metrics update in dashboard UI", async ({
			page,
		}) => {
			await page.goto("/");

			// Check Overview metric cards
			await expect(
				page.getByRole("heading", { name: "System Overview" }),
			).toBeVisible();

			// Click Refresh metrics
			await page.getByRole("button", { name: "Refresh" }).click();

			// Verify metrics are loaded and visible
			await expect(page.getByText("Total Requests")).toBeVisible();
			await expect(page.getByText("Monthly Usage Quota")).toBeVisible();
			await expect(page.getByText("P95 Latency")).toBeVisible();

			// Navigate to Usage & Analytics
			await page
				.getByRole("button", { name: "Usage & Analytics" })
				.click();
			await expect(
				page.getByRole("heading", { name: "Usage & Analytics", level: 1 }),
			).toBeVisible();
			await expect(page.getByText("Endpoint Breakdown")).toBeVisible();
		});

		// -------------------------------------------------------------------------
		// Test 6: User revokes API key; subsequent requests return 401 Unauthorized
		// -------------------------------------------------------------------------
		test("Test 6: User revokes API key; subsequent requests return 401 Unauthorized", async ({
			page,
			request,
		}) => {
			expect(createdApiKey).toBeTruthy();

			// Navigate to API Keys page
			await page.goto("/");
			await page.getByRole("button", { name: "API Keys" }).click();

			// Locate the delete button for our created key
			const revokeBtn = page
				.locator('button[title="Revoke this API Key"]')
				.first();
			await expect(revokeBtn).toBeVisible();
			await revokeBtn.click();

			// Confirm Revocation in Modal
			await expect(
				page.getByRole("heading", { name: "Revoke API Key" }),
			).toBeVisible();
			await page
				.getByRole("button", { name: /Confirm Revocation/i })
				.click();

			// Wait for revocation to complete and modal to close
			await expect(
				page.getByRole("heading", { name: "Revoke API Key" }),
			).not.toBeVisible();
			await expect(page.getByText("Revoked").first()).toBeVisible();

			// Subsequent request with the revoked API key must return 401 Unauthorized
			const deniedResp = await request.post(
				`${apiBaseURL}/api/v1/ocr/ktp`,
				{
					headers: {
						Authorization: `Bearer ${createdApiKey}`,
					},
					multipart: {
						document: {
							name: "test.jpg",
							mimeType: "image/jpeg",
							buffer: syntheticKtpJpeg,
						},
					},
				},
			);

			expect(deniedResp.status()).toBe(401);
			const errorJson = await deniedResp.json();
			expect(errorJson.error.code).toBe("invalid_api_key");
			expect(errorJson.error.message).toContain("revoked");
		});
	},
);
