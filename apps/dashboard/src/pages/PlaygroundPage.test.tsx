import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider } from "../context/AuthContext";
import { api } from "../services/api";
import type {
	InvoiceResponse,
	KKResponse,
	KTPResponse,
	NPWPResponse,
	PassportResponse,
} from "../types/api";
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
			.mockResolvedValue(mockPassportResult as unknown as PassportResponse);

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
			.mockResolvedValue(mockNpwpResult as unknown as NPWPResponse);

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

	it("tests synthetic fixture KK OCR execution when switched to KK tab", async () => {
		const mockKKResult = {
			id: "ocr_kk_play_1",
			document_type: "kk",
			status: "completed",
			confidence: 0.99,
			processing: {
				latency_ms: 145,
			},
			data: {
				nomor_kk: "3273012301200001",
				kepala_keluarga: "BUDI SANTOSO",
				alamat: "JL. MERDEKA NO. 45",
				rt_rw: "005/002",
				kelurahan_desa: "BABAKAN",
				kecamatan: "COBLONG",
				kabupaten_kota: "KOTA BANDUNG",
				provinsi: "JAWA BARAT",
				tanggal_dikeluarkan: "2020-01-23",
				anggota_keluarga: [
					{
						nama: "BUDI SANTOSO",
						nik: "3273011508850001",
						jenis_kelamin: "LAKI-LAKI",
						tempat_lahir: "BANDUNG",
						tanggal_lahir: "1985-08-15",
						agama: "ISLAM",
						pendidikan: "S1",
						jenis_pekerjaan: "KARYAWAN SWASTA",
						status_perkawinan: "KAWIN",
						status_hubungan: "KEPALA KELUARGA",
						kewarganegaraan: "WNI",
					},
				],
			},
			field_confidence: {
				nomor_kk: 1.0,
				kepala_keluarga: 0.99,
			},
		};

		const kkSpy = vi
			.spyOn(api, "executeKKOCR")
			.mockResolvedValue(mockKKResult as unknown as KKResponse);

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		// Switch to KK tab
		const kkTabBtn = screen.getByRole("button", { name: /^KK$/i });
		fireEvent.click(kkTabBtn);

		// Click "Load Synthetic Fixture"
		const fixtureBtn = screen.getByRole("button", {
			name: /Load Synthetic Fixture/i,
		});
		fireEvent.click(fixtureBtn);

		await waitFor(() => {
			expect(kkSpy).toHaveBeenCalledTimes(1);
			expect(screen.getByText(/Normalized Field Verification/)).toBeDefined();
			expect(screen.getByText("3273012301200001")).toBeDefined();
			expect(screen.getByText("JL. MERDEKA NO. 45")).toBeDefined();
			expect(screen.getByText("KEPALA KELUARGA")).toBeDefined();
			expect(screen.getByText(/99% Confidence/)).toBeDefined();
		});
	});

	it("tests synthetic fixture Invoice OCR execution when switched to Invoice tab", async () => {
		const mockInvoiceResult = {
			id: "ocr_inv_play_1",
			document_type: "invoice",
			status: "completed",
			confidence: 0.99,
			processing: {
				latency_ms: 150,
			},
			data: {
				invoice_number: "INV/2023/11/001",
				invoice_date: "2023-11-20",
				due_date: "2023-12-20",
				seller_name: "PT TEKNOLOGI MAJU JAYA",
				seller_npwp: "01.234.567.8-901.000",
				seller_address: "JL. SUDIRMAN NO. 123",
				buyer_name: "PT GLOBAL SOLUSI MANDIRI",
				buyer_npwp: "02.345.678.9-012.000",
				buyer_address: "JL. THAMRIN NO. 45",
				currency: "IDR",
				subtotal: 10000000,
				discount: 500000,
				dpp: 9500000,
				ppn: 1045000,
				grand_total: 10545000,
				line_items: [
					{
						description: "Cloud Server Hosting",
						quantity: 2,
						unit_price: 5000000,
						total_price: 10000000,
					},
				],
			},
			field_confidence: {
				invoice_number: 1.0,
				seller_name: 0.99,
			},
		};

		const invoiceSpy = vi
			.spyOn(api, "executeInvoiceOCR")
			.mockResolvedValue(mockInvoiceResult as unknown as InvoiceResponse);

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		// Switch to Invoice tab
		const invoiceTabBtn = screen.getByRole("button", { name: /^Invoice$/i });
		fireEvent.click(invoiceTabBtn);

		// Click "Load Synthetic Fixture"
		const fixtureBtn = screen.getByRole("button", {
			name: /Load Synthetic Fixture/i,
		});
		fireEvent.click(fixtureBtn);

		await waitFor(() => {
			expect(invoiceSpy).toHaveBeenCalledTimes(1);
			expect(screen.getByText(/Normalized Field Verification/)).toBeDefined();
			expect(screen.getByText("INV/2023/11/001")).toBeDefined();
			expect(screen.getByText("PT TEKNOLOGI MAJU JAYA")).toBeDefined();
			expect(screen.getByText("Cloud Server Hosting")).toBeDefined();
			expect(screen.getByText(/99% Confidence/)).toBeDefined();
		});
	});

	it("runs via session cookie when apiKey is missing", async () => {
		const mockOcrResult = {
			id: "ocr_play_cookie",
			status: "completed",
			confidence: 0.99,
			latency_ms: 115,
			data: {
				nik: "3273012345670001",
				nama: "COOKIE SESSION",
				kewarganegaraan: "WNI",
			},
			field_confidence: {
				nik: 1.0,
			},
		};
		const ocrSpy = vi
			.spyOn(api, "executeKTPOCR")
			.mockResolvedValue(mockOcrResult as unknown as KTPResponse);
		const onNavigate = vi.fn();

		renderWithAuth(<PlaygroundPage onNavigate={onNavigate} />, {
			initialOrg: TEST_ORG,
		});

		expect(
			screen.getByText(/API Key hanya dibutuhkan untuk pemanggilan/),
		).toBeDefined();

		// Click "Kelola API Key"
		fireEvent.click(screen.getByText("Kelola API Key"));
		expect(onNavigate).toHaveBeenCalledWith("keys");

		// Click "Load Synthetic Fixture" — proceeds without a key (cookie auth)
		const fixtureBtn = screen.getByRole("button", {
			name: /Load Synthetic Fixture/i,
		});
		fireEvent.click(fixtureBtn);

		await waitFor(() => {
			expect(ocrSpy).toHaveBeenCalledTimes(1);
			expect(ocrSpy).toHaveBeenCalledWith(expect.anything(), undefined);
			expect(screen.getByText("3273012345670001")).toBeDefined();
			expect(screen.getByText("COOKIE SESSION")).toBeDefined();
		});
	});

	it("keeps the previous result visible while the next scan runs", async () => {
		const firstResult = {
			id: "ocr_play_first",
			status: "completed",
			confidence: 0.99,
			latency_ms: 115,
			data: {
				nik: "3273012345670001",
				nama: "HASIL PERTAMA",
				kewarganegaraan: "WNI",
			},
			field_confidence: {
				nik: 1.0,
			},
		};
		const secondResult = {
			id: "ocr_play_second",
			status: "completed",
			confidence: 0.97,
			latency_ms: 130,
			data: {
				nik: "3273012345670002",
				nama: "HASIL KEDUA",
				kewarganegaraan: "WNI",
			},
			field_confidence: {
				nik: 1.0,
			},
		};

		let resolveSecond!: (value: KTPResponse) => void;
		const ocrSpy = vi
			.spyOn(api, "executeKTPOCR")
			.mockResolvedValueOnce(firstResult as unknown as KTPResponse)
			.mockImplementationOnce(
				() =>
					new Promise<KTPResponse>((resolve) => {
						resolveSecond = resolve;
					}),
			);

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		const fixtureBtn = screen.getByRole("button", {
			name: /Load Synthetic Fixture/i,
		});
		fireEvent.click(fixtureBtn);
		await waitFor(() => {
			expect(screen.getByText("3273012345670001")).toBeDefined();
		});

		fireEvent.click(fixtureBtn);
		await waitFor(() => {
			expect(screen.getByText("Memperbarui…")).toBeDefined();
		});
		expect(ocrSpy).toHaveBeenCalledTimes(2);
		expect(screen.getByText("3273012345670001")).toBeDefined();
		expect(screen.queryByText("3273012345670002")).toBeNull();

		resolveSecond(secondResult as unknown as KTPResponse);
		await waitFor(() => {
			expect(screen.getByText("3273012345670002")).toBeDefined();
		});
	});

	it("keeps the previous result when the next scan fails", async () => {
		const firstResult = {
			id: "ocr_play_first",
			status: "completed",
			confidence: 0.99,
			latency_ms: 115,
			data: {
				nik: "3273012345670001",
				nama: "HASIL PERTAMA",
				kewarganegaraan: "WNI",
			},
			field_confidence: {
				nik: 1.0,
			},
		};

		vi.spyOn(api, "executeKTPOCR")
			.mockResolvedValueOnce(firstResult as unknown as KTPResponse)
			.mockRejectedValueOnce(new Error("Upstream Gemini timeout"));

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		const fixtureBtn = screen.getByRole("button", {
			name: /Load Synthetic Fixture/i,
		});
		fireEvent.click(fixtureBtn);
		await waitFor(() => {
			expect(screen.getByText("3273012345670001")).toBeDefined();
		});

		fireEvent.click(fixtureBtn);
		await waitFor(() => {
			expect(screen.getByText("Upstream Gemini timeout")).toBeDefined();
		});
		expect(screen.getByText("3273012345670001")).toBeDefined();
	});

	const stubObjectURLs = () => {
		let n = 0;
		const createMock = vi.fn(() => {
			n += 1;
			return `blob:mock-${n}`;
		});
		const revokeMock = vi.fn((_url: string) => {});
		window.URL.createObjectURL = createMock;
		window.URL.revokeObjectURL = revokeMock;
		return { createMock, revokeMock };
	};

	const minimalKtpResult = {
		id: "ocr_preview_1",
		status: "completed",
		confidence: 0.9,
		latency_ms: 10,
		data: {},
		field_confidence: {},
	};

	const zoomFixtureResult = {
		id: "ocr_zoom_1",
		status: "completed",
		confidence: 0.9,
		latency_ms: 10,
		data: {
			nik: "3273012345670099",
			nama: "ZOOM COMPARE",
			kewarganegaraan: "WNI",
		},
		field_confidence: {
			nik: 1.0,
		},
	};

	const uploadFile = (name: string, content: string) => {
		const fileInput = document.getElementById(
			"ktp-file-input",
		) as HTMLInputElement;
		fireEvent.change(fileInput, {
			target: { files: [new File([content], name, { type: "image/jpeg" })] },
		});
	};

	it("shows a source image preview for the uploaded document", async () => {
		const { createMock } = stubObjectURLs();
		vi.spyOn(api, "executeKTPOCR").mockResolvedValue(
			minimalKtpResult as unknown as KTPResponse,
		);

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		uploadFile("ktp_preview.jpg", "bytes-1");

		await waitFor(() => {
			const img = screen.getByAltText("Pratinjau ktp_preview.jpg");
			expect(img.getAttribute("src")).toBe("blob:mock-1");
		});
		expect(createMock).toHaveBeenCalledTimes(1);
	});

	it("replaces the preview on the next scan and revokes the old URL", async () => {
		const { revokeMock } = stubObjectURLs();
		vi.spyOn(api, "executeKTPOCR").mockResolvedValue(
			minimalKtpResult as unknown as KTPResponse,
		);

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		uploadFile("ktp_satu.jpg", "bytes-1");
		await waitFor(() => {
			expect(
				screen.getByAltText("Pratinjau ktp_satu.jpg").getAttribute("src"),
			).toBe("blob:mock-1");
		});

		uploadFile("ktp_dua.jpg", "bytes-2");
		await waitFor(() => {
			expect(
				screen.getByAltText("Pratinjau ktp_dua.jpg").getAttribute("src"),
			).toBe("blob:mock-2");
		});
		expect(revokeMock).toHaveBeenCalledWith("blob:mock-1");
	});

	it("expands the preview inline while the result stays visible", async () => {
		stubObjectURLs();
		vi.spyOn(api, "executeKTPOCR").mockResolvedValue(
			zoomFixtureResult as unknown as KTPResponse,
		);

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		uploadFile("ktp_zoom.jpg", "bytes-zoom");
		await waitFor(() => {
			expect(screen.getByAltText("Pratinjau ktp_zoom.jpg")).toBeDefined();
			expect(screen.getByText("3273012345670099")).toBeDefined();
		});

		fireEvent.click(screen.getByRole("button", { name: "Perbesar" }));

		await waitFor(() => {
			expect(
				screen.getByAltText("Pratinjau diperbesar ktp_zoom.jpg"),
			).toBeDefined();
		});
		expect(screen.queryByRole("dialog")).toBeNull();
		expect(screen.getByText("3273012345670099")).toBeDefined();
	});

	it("zooms in, resets, and collapses the inline viewer", async () => {
		stubObjectURLs();
		vi.spyOn(api, "executeKTPOCR").mockResolvedValue(
			zoomFixtureResult as unknown as KTPResponse,
		);

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		uploadFile("ktp_zoom2.jpg", "bytes-zoom2");
		await waitFor(() => {
			expect(screen.getByAltText("Pratinjau ktp_zoom2.jpg")).toBeDefined();
		});
		fireEvent.click(screen.getByRole("button", { name: "Perbesar" }));
		const zoomed = await screen.findByAltText(
			"Pratinjau diperbesar ktp_zoom2.jpg",
		);
		expect(zoomed.getAttribute("style")).toContain("scale(1)");

		fireEvent.click(screen.getByRole("button", { name: "Perbesar gambar" }));
		await waitFor(() => {
			expect(zoomed.getAttribute("style")).toContain("scale(1.5)");
		});
		expect(screen.getByText("150%")).toBeDefined();

		fireEvent.click(screen.getByRole("button", { name: "Atur ulang zoom" }));
		await waitFor(() => {
			expect(zoomed.getAttribute("style")).toContain("scale(1)");
		});

		fireEvent.click(screen.getByRole("button", { name: "Perkecil" }));
		await waitFor(() => {
			expect(
				screen.queryByAltText("Pratinjau diperbesar ktp_zoom2.jpg"),
			).toBeNull();
			expect(screen.getByAltText("Pratinjau ktp_zoom2.jpg")).toBeDefined();
		});
	});

	it("binds the file to its result in the comparison header", async () => {
		vi.spyOn(api, "executeKTPOCR").mockResolvedValue(
			zoomFixtureResult as unknown as KTPResponse,
		);

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		expect(screen.getByText("Belum ada dokumen")).toBeDefined();
		expect(screen.getByText("Siap")).toBeDefined();

		uploadFile("ktp_compare.jpg", "bytes-compare");

		await waitFor(() => {
			expect(screen.getByText("ktp_compare.jpg")).toBeDefined();
			expect(screen.getByText("Selesai")).toBeDefined();
			expect(screen.getByText("3273012345670099")).toBeDefined();
		});
	});

	it("copies the result payload to the clipboard", async () => {
		const writeText = vi.fn(async (_text: string) => {});
		Object.defineProperty(window.navigator, "clipboard", {
			value: { writeText },
			configurable: true,
		});
		vi.spyOn(api, "executeKTPOCR").mockResolvedValue(
			zoomFixtureResult as unknown as KTPResponse,
		);

		renderWithAuth(<PlaygroundPage />, {
			initialOrg: TEST_ORG,
			initialApiKey: "lensio_live_testkey123",
		});

		uploadFile("ktp_copy.jpg", "bytes-copy");
		await waitFor(() => {
			expect(screen.getByText("3273012345670099")).toBeDefined();
		});

		fireEvent.click(screen.getByRole("button", { name: "Salin JSON hasil" }));

		await waitFor(() => {
			expect(writeText).toHaveBeenCalledTimes(1);
		});
		const payload = writeText.mock.calls[0][0] as string;
		expect(payload).toContain("3273012345670099");
		expect(screen.getByText("Disalin")).toBeDefined();
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
