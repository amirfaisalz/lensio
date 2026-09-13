import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../context/AuthContext";
import { api } from "../services/api";
import { PlaygroundPage } from "./PlaygroundPage";

const TEST_ORG = {
	id: "org-test-uuid",
	name: "Acme Corp",
	slug: "acme",
	planCode: "free",
};

const renderWithAuth = (
	ui: React.ReactElement,
	options?: {
		initialOrg?: typeof TEST_ORG | null;
		initialApiKey?: string | null;
	},
) => {
	return render(
		<AuthProvider
			initialOrg={options?.initialOrg}
			initialApiKey={options?.initialApiKey}
		>
			{ui}
		</AuthProvider>,
	);
};

describe("PlaygroundPage", () => {
	beforeEach(() => {
		api.setApiKey(null);
		localStorage.clear();
		vi.restoreAllMocks();
	});

	it("renders empty organization state when currentOrg is null", () => {
		const onNavigate = vi.fn();
		renderWithAuth(<PlaygroundPage onNavigate={onNavigate} />);

		expect(screen.getByText("Live OCR Playground")).toBeDefined();
		expect(screen.getByText("Pilih atau Buat Organisasi")).toBeDefined();

		const btn = screen.getByRole("button", {
			name: /Buka Pengaturan Organisasi & API Key/i,
		});
		fireEvent.click(btn);
		expect(onNavigate).toHaveBeenCalledWith("keys");
	});

	it("tests synthetic fixture KTP execution when organization and apiKey exist", async () => {
		const mockOcrResult = {
			id: "ocr_play_1",
			status: "completed",
			confidence: 0.99,
			latency_ms: 115,
			data: {
				nik: "3273012345670001",
				nama: "JOKO WIDODO SYNTHETIC",
				tempat_lahir: "SURAKARTA",
				tanggal_lahir: "1961-06-21",
				jenis_kelamin: "LAKI-LAKI",
				alamat: "JL. VETERAN NO. 1",
				rt_rw: "001/001",
				kelurahan: "MANAHAN",
				kecamatan: "BANJARSARI",
				agama: "ISLAM",
				status_perkawinan: "KAWIN",
				pekerjaan: "PEGAWAI NEGERI",
				kewarganegaraan: "WNI",
			},
			field_confidence: {
				nik: 1.0,
				nama: 0.99,
			},
		};

		const ocrSpy = vi
			.spyOn(api, "executeKTPOCR")
			.mockResolvedValue(mockOcrResult);

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		expect(screen.getByText("Live OCR Playground")).toBeDefined();
		expect(screen.queryByText("Pilih atau Buat Organisasi")).toBeNull();

		// Click "Load Synthetic Fixture"
		const fixtureBtn = screen.getByRole("button", {
			name: /Load Synthetic Fixture/i,
		});
		fireEvent.click(fixtureBtn);

		await waitFor(() => {
			expect(ocrSpy).toHaveBeenCalledTimes(1);
			expect(screen.getByText(/Normalized Field Verification/)).toBeDefined();
			expect(screen.getByText("3273012345670001")).toBeDefined();
			expect(screen.getByText("JOKO WIDODO SYNTHETIC")).toBeDefined();
			expect(screen.getByText(/99% Confidence/)).toBeDefined();
		});
	});

	it("tests synthetic fixture SIM OCR execution when switched to SIM tab", async () => {
		const mockSimResult = {
			id: "ocr_sim_play_1",
			document_type: "sim",
			status: "completed",
			confidence: 0.98,
			processing: {
				latency_ms: 125,
			},
			data: {
				nomor_sim: "123456789012",
				golongan: "A",
				nama: "JOKO WIDODO SYNTHETIC",
				alamat: "JL. VETERAN NO. 1",
				rt_rw: "001/001",
				kelurahan: "MANAHAN",
				kecamatan: "BANJARSARI",
				kota: "SURAKARTA",
				pekerjaan: "SWASTA",
				tempat_lahir: "SURAKARTA",
				tanggal_lahir: "1961-06-21",
				jenis_kelamin: "PRIA",
				golongan_darah: "O",
				masa_berlaku: "2029-06-21",
				polda: "POLDA JAWA TENGAH",
			},
			field_confidence: {
				nomor_sim: 1.0,
				golongan: 1.0,
				nama: 0.98,
			},
		};

		const simSpy = vi
			.spyOn(api, "executeSIMOCR")
			.mockResolvedValue(mockSimResult);

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		// Switch to SIM tab
		const simTabBtn = screen.getByRole("button", { name: /^SIM$/i });
		fireEvent.click(simTabBtn);

		// Click "Load Synthetic Fixture"
		const fixtureBtn = screen.getByRole("button", {
			name: /Load Synthetic Fixture/i,
		});
		fireEvent.click(fixtureBtn);

		await waitFor(() => {
			expect(simSpy).toHaveBeenCalledTimes(1);
			expect(screen.getByText(/Normalized Field Verification/)).toBeDefined();
			expect(screen.getByText("123456789012")).toBeDefined();
			expect(screen.getByText("POLDA JAWA TENGAH")).toBeDefined();
			expect(screen.getByText(/98% Confidence/)).toBeDefined();
		});
	});

	it("tests synthetic fixture Passport OCR execution when switched to Passport tab", async () => {
		const mockPassportResult = {
			id: "ocr_pass_play_1",
			document_type: "passport",
			status: "completed",
			confidence: 0.99,
			processing: {
				latency_ms: 135,
			},
			data: {
				passport_number: "X1234567",
				full_name: "BUDI SANTOSO",
				nationality: "IDN",
				date_of_birth: "1990-01-01",
				gender: "LAKI-LAKI",
				expiry_date: "2030-01-01",
				issuing_country: "IDN",
				issuing_office: "JAKARTA SELATAN",
				mrz_line1: "P<IDNSANTOSO<<BUDI<<<<<<<<<<<<<<<<<<<<<<<<<<",
				mrz_line2: "X1234567<7IDN9001011M3001019<<<<<<<<<<<<<<<2",
			},
			field_confidence: {
				passport_number: 1.0,
				full_name: 0.99,
			},
		};

		const passportSpy = vi
			.spyOn(api, "executePassportOCR")
			.mockResolvedValue(mockPassportResult as any);

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		// Switch to Passport tab
		const passportTabBtn = screen.getByRole("button", { name: /^Passport$/i });
		fireEvent.click(passportTabBtn);

		// Click "Load Synthetic Fixture"
		const fixtureBtn = screen.getByRole("button", {
			name: /Load Synthetic Fixture/i,
		});
		fireEvent.click(fixtureBtn);

		await waitFor(() => {
			expect(passportSpy).toHaveBeenCalledTimes(1);
			expect(screen.getByText(/Normalized Field Verification/)).toBeDefined();
			expect(screen.getByText("X1234567")).toBeDefined();
			expect(screen.getByText("JAKARTA SELATAN")).toBeDefined();
			expect(screen.getByText(/99% Confidence/)).toBeDefined();
		});
	});

	it("tests synthetic fixture NPWP OCR execution when switched to NPWP tab", async () => {
		const mockNpwpResult = {
			id: "ocr_npwp_play_1",
			document_type: "npwp",
			status: "completed",
			confidence: 0.99,
			processing: {
				latency_ms: 120,
			},
			data: {
				npwp: "09.254.294.3-407.000",
				nama: "PT LIMA SEKAWAN SYNTHETIC",
				nik: "3171010101900001",
				alamat: "JL. JEND. SUDIRMAN KAV. 52-53",
				kelurahan: "SENAYAN",
				kecamatan: "KEBAYORAN BARU",
				kota_kabupaten: "JAKARTA SELATAN",
				provinsi: "DKI JAKARTA",
				kpp: "KPP PRATAMA JAKARTA KEBAYORAN BARU DUA",
				tanggal_daftar: "2015-08-17",
			},
			field_confidence: {
				npwp: 1.0,
				nama: 0.99,
			},
		};

		const npwpSpy = vi
			.spyOn(api, "executeNPWPOCR")
			.mockResolvedValue(mockNpwpResult as any);

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		// Switch to NPWP tab
		const npwpTabBtn = screen.getByRole("button", { name: /^NPWP$/i });
		fireEvent.click(npwpTabBtn);

		// Click "Load Synthetic Fixture"
		const fixtureBtn = screen.getByRole("button", {
			name: /Load Synthetic Fixture/i,
		});
		fireEvent.click(fixtureBtn);

		await waitFor(() => {
			expect(npwpSpy).toHaveBeenCalledTimes(1);
			expect(screen.getByText(/Normalized Field Verification/)).toBeDefined();
			expect(screen.getByText("09.254.294.3-407.000")).toBeDefined();
			expect(screen.getByText("PT LIMA SEKAWAN SYNTHETIC")).toBeDefined();
			expect(screen.getByText(/99% Confidence/)).toBeDefined();
		});
	});

	it("shows warning and blocks execution when apiKey is missing", async () => {
		const ocrSpy = vi.spyOn(api, "executeKTPOCR");
		const onNavigate = vi.fn();

		renderWithAuth(<PlaygroundPage onNavigate={onNavigate} />, {
			initialOrg: TEST_ORG,
		});

		expect(
			screen.getByText(
				/API Key diperlukan untuk menguji OCR di playground ini/,
			),
		).toBeDefined();

		// Click "Kelola API Key"
		fireEvent.click(screen.getByText("Kelola API Key"));
		expect(onNavigate).toHaveBeenCalledWith("keys");

		// Click "Load Synthetic Fixture"
		const fixtureBtn = screen.getByRole("button", {
			name: /Load Synthetic Fixture/i,
		});
		fireEvent.click(fixtureBtn);

		await waitFor(() => {
			expect(ocrSpy).not.toHaveBeenCalled();
			expect(
				screen.getByText(/API Key aktif diperlukan untuk menjalankan OCR/),
			).toBeDefined();
		});
	});

	it("handles file upload error in playground", async () => {
		vi.spyOn(api, "executeKTPOCR").mockRejectedValue(
			new Error("Image blur score too low for OCR extraction"),
		);

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		const fileInput = document.getElementById(
			"ktp-file-input",
		) as HTMLInputElement;
		expect(fileInput).toBeDefined();

		const file = new File(["dummy content"], "test_ktp.jpg", {
			type: "image/jpeg",
		});
		fireEvent.change(fileInput, { target: { files: [file] } });

		await waitFor(() => {
			expect(
				screen.getByText("Image blur score too low for OCR extraction"),
			).toBeDefined();
		});
	});
});
