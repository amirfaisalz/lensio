import {
	AlertTriangle,
	Building2,
	FileCheck,
	KeyRound,
	RefreshCw,
	Upload,
} from "lucide-react";
import type React from "react";
import { useState } from "react";
import type { NavigationPage } from "../components/layout/Sidebar";
import { useAuth } from "../context/AuthContext";
import { api } from "../services/api";
import type { KTPResponse, SIMResponse } from "../types/api";

// 400x250 valid synthetic KTP JPEG fixture
const SYNTHETIC_KTP_BASE64 =
	"/9j/2wCEAAMCAgMCAgMDAwMEAwMEBQgFBQQEBQoHBwYIDAoMDAsKCwsNDhIQDQ4RDgsLEBYQERMUFRUVDA8XGBYUGBIUFRQBAwQEBQQFCQUFCRQNCw0UFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFP/AABEIAPoBkAMBIgACEQEDEQH/xAGiAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgsQAAIBAwMCBAMFBQQEAAABfQECAwAEEQUSITFBBhNRYQcicRQygZGhCCNCscEVUtHwJDNicoIJChYXGBkaJSYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6g4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2drh4uPk5ebn6Onq8fLz9PX29/j5+gEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoLEQACAQIEBAMEBwUEBAABAncAAQIDEQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEKFiQ04SXxFxgZGiYnKCkqNTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqCg4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2dri4+Tl5ufo6ery8/T19vf4+fr/2gAMAwEAAhEDEQA/AP0Fooor6M+fCiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKAP//Z";

function base64ToUint8Array(base64: string): Uint8Array {
	const binaryString = atob(base64);
	const bytes = new Uint8Array(binaryString.length);
	for (let i = 0; i < binaryString.length; i++) {
		bytes[i] = binaryString.charCodeAt(i);
	}
	return bytes;
}

export interface PlaygroundPageProps {
	onNavigate?: (page: NavigationPage) => void;
}

export const PlaygroundPage: React.FC<PlaygroundPageProps> = ({
	onNavigate,
}) => {
	const { currentOrg, apiKey } = useAuth();

	const [docType, setDocType] = useState<"ktp" | "sim">("ktp");
	const [ocrLoading, setOcrLoading] = useState(false);
	const [ocrResult, setOcrResult] = useState<KTPResponse | SIMResponse | null>(
		null,
	);
	const [ocrError, setOcrError] = useState<string | null>(null);
	const [selectedFileName, setSelectedFileName] = useState<string | null>(null);

	const handleFileUpload = async (file: File) => {
		if (!apiKey) {
			setOcrError(
				"API Key aktif diperlukan untuk menjalankan OCR. Silakan buat atau aktifkan API Key di menu API Keys.",
			);
			return;
		}

		try {
			setSelectedFileName(
				file.name ||
					(docType === "sim" ? "sim_document.jpg" : "ktp_document.jpg"),
			);
			setOcrLoading(true);
			setOcrError(null);
			const res =
				docType === "sim"
					? await api.executeSIMOCR(file, apiKey)
					: await api.executeKTPOCR(file, apiKey);
			setOcrResult(res);
		} catch (err) {
			setOcrError(err instanceof Error ? err.message : "OCR execution failed");
			setOcrResult(null);
		} finally {
			setOcrLoading(false);
		}
	};

	// Helper to load synthetic sample image fixture
	const handleLoadSyntheticSample = async () => {
		try {
			const filename =
				docType === "sim"
					? "synthetic_sim_fixture.jpg"
					: "synthetic_ktp_fixture.jpg";
			setSelectedFileName(filename);
			setOcrLoading(true);
			setOcrError(null);

			let bytes = base64ToUint8Array(SYNTHETIC_KTP_BASE64);
			if (docType === "sim") {
				const marker = new TextEncoder().encode("MOCK_SIM_DOC");
				const combined = new Uint8Array(bytes.length + marker.length);
				combined.set(bytes);
				combined.set(marker, bytes.length);
				bytes = combined;
			}

			const file = new File([bytes as unknown as BlobPart], filename, {
				type: "image/jpeg",
			});
			await handleFileUpload(file);
		} catch (err) {
			setOcrError(
				err instanceof Error
					? err.message
					: "Failed to generate synthetic sample",
			);
			setOcrLoading(false);
		}
	};

	if (!currentOrg) {
		return (
			<div className="space-y-6 animate-in fade-in duration-200">
				<div>
					<h2 className="text-xl font-bold text-slate-900 tracking-tight">
						Live OCR Playground
					</h2>
					<p className="text-xs text-slate-500">
						Interactive test harness for Indonesian identity documents.
					</p>
				</div>

				<div className="bg-white rounded-xl border border-slate-200 p-8 text-center max-w-lg mx-auto shadow-xs">
					<div className="w-12 h-12 rounded-xl bg-[#E7F3FF] text-[#1877F2] flex items-center justify-center mx-auto mb-4">
						<Building2 className="w-6 h-6" />
					</div>
					<h3 className="text-base font-bold text-slate-900 mb-1">
						Pilih atau Buat Organisasi
					</h3>
					<p className="text-xs text-slate-500 mb-6 leading-relaxed">
						Playground memerlukan organisasi aktif untuk pengujian OCR dan
						pelacakan kuota bulanan.
					</p>
					<button
						type="button"
						onClick={() => onNavigate?.("keys")}
						className="px-4 py-2 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] rounded-lg transition-colors cursor-pointer"
					>
						Buka Pengaturan Organisasi & API Key
					</button>
				</div>
			</div>
		);
	}

	return (
		<div className="space-y-6 animate-in fade-in duration-200">
			{/* Page Header */}
			<div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
				<div>
					<h2 className="text-xl font-bold text-slate-900 tracking-tight">
						Live OCR Playground
					</h2>
					<p className="text-xs text-slate-500">
						Interactive test harness for Indonesian KTP & SIM document
						extraction with synthetic fixtures.
					</p>
				</div>

				{/* Document Switcher & Fixture Button */}
				<div className="flex items-center gap-2.5">
					<div className="flex items-center bg-slate-200/70 p-0.5 rounded-lg text-xs font-semibold">
						<button
							type="button"
							onClick={() => {
								setDocType("ktp");
								setOcrResult(null);
								setSelectedFileName(null);
								setOcrError(null);
							}}
							className={`px-3 py-1 rounded-md transition-all cursor-pointer ${
								docType === "ktp"
									? "bg-white text-[#1877F2] shadow-xs"
									: "text-slate-600 hover:text-slate-900"
							}`}
						>
							KTP
						</button>
						<button
							type="button"
							onClick={() => {
								setDocType("sim");
								setOcrResult(null);
								setSelectedFileName(null);
								setOcrError(null);
							}}
							className={`px-3 py-1 rounded-md transition-all cursor-pointer ${
								docType === "sim"
									? "bg-white text-[#1877F2] shadow-xs"
									: "text-slate-600 hover:text-slate-900"
							}`}
						>
							SIM
						</button>
					</div>
					<button
						type="button"
						onClick={handleLoadSyntheticSample}
						disabled={ocrLoading}
						className="inline-flex items-center justify-center gap-1.5 px-3.5 py-1.5 text-xs font-semibold text-[#1877F2] bg-[#E7F3FF] hover:bg-[#d5eaff] rounded-lg transition-colors cursor-pointer"
					>
						<FileCheck className="w-3.5 h-3.5" />
						<span>Load Synthetic Fixture</span>
					</button>
				</div>
			</div>

			{/* Missing API Key Warning */}
			{!apiKey && (
				<div className="p-4 bg-amber-50 border border-amber-200 rounded-xl text-xs text-amber-800 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 shadow-xs">
					<div className="flex items-center gap-2.5">
						<KeyRound className="w-4 h-4 text-amber-600 shrink-0" />
						<span>
							API Key diperlukan untuk menguji OCR di playground ini. Silakan
							buat atau aktifkan API Key organisasi Anda.
						</span>
					</div>
					<button
						type="button"
						onClick={() => onNavigate?.("keys")}
						className="font-semibold text-[#1877F2] hover:underline cursor-pointer shrink-0"
					>
						Kelola API Key
					</button>
				</div>
			)}

			{/* Main Playground Workspace Card */}
			<div className="bg-white rounded-xl border border-slate-200 shadow-xs overflow-hidden">
				<div className="p-4 sm:p-6 grid grid-cols-1 lg:grid-cols-2 gap-4 sm:gap-6">
					{/* Left: File Drop Zone */}
					<div>
						<label
							htmlFor="ktp-file-input"
							className="border-2 border-dashed border-slate-300 hover:border-[#1877F2] rounded-xl p-8 flex flex-col items-center justify-center text-center cursor-pointer transition-colors bg-slate-50/50 hover:bg-[#F0F2F5]/50 min-h-[220px]"
						>
							<Upload className="w-8 h-8 text-slate-400 mb-3" />
							<p className="text-sm font-semibold text-slate-700">
								Click to upload or drag & drop
							</p>
							<p className="text-xs text-slate-500 mt-1">
								Upload Indonesian {docType === "sim" ? "SIM" : "KTP"} image
								(JPEG, PNG, max 5MB)
							</p>
							<input
								id="ktp-file-input"
								type="file"
								accept="image/jpeg,image/png,image/webp"
								className="hidden"
								onChange={(e) => {
									if (e.target.files?.[0]) {
										handleFileUpload(e.target.files[0]);
									}
								}}
							/>
						</label>

						{selectedFileName && (
							<div className="mt-3 flex items-center justify-between px-3 py-2 bg-slate-50 rounded-lg border border-slate-200 text-xs text-slate-600">
								<span className="truncate font-mono">{selectedFileName}</span>
								{ocrLoading && (
									<RefreshCw className="w-3.5 h-3.5 animate-spin text-[#1877F2]" />
								)}
							</div>
						)}

						{ocrError && (
							<div className="mt-3 p-3 bg-rose-50 border border-rose-200 rounded-lg text-xs text-rose-700 flex items-start gap-2">
								<AlertTriangle className="w-4 h-4 text-rose-500 shrink-0 mt-0.5" />
								<span>
									<strong>Error:</strong> {ocrError}
								</span>
							</div>
						)}
					</div>

					{/* Right: Results JSON Viewer */}
					<div className="bg-slate-900 text-slate-100 rounded-xl p-4 flex flex-col font-mono text-[11px] sm:text-xs overflow-hidden min-h-[260px] max-h-[420px]">
						<div className="flex items-center justify-between pb-2 mb-2 border-b border-slate-800 text-slate-400 text-[11px]">
							<span>RESPONSE PAYLOAD</span>
							{ocrResult && (
								<span className="text-emerald-400">
									{"processing" in ocrResult
										? ocrResult.processing.latency_ms
										: ocrResult.latency_ms}
									ms · {Math.round(ocrResult.confidence * 100)}% Confidence
								</span>
							)}
						</div>

						<div className="flex-1 overflow-y-auto">
							{ocrLoading ? (
								<div className="h-full flex items-center justify-center text-slate-500 py-16">
									<RefreshCw className="w-5 h-5 animate-spin mr-2 text-[#1877F2]" />
									<span>
										Processing {docType === "sim" ? "SIM" : "KTP"} OCR
										extraction...
									</span>
								</div>
							) : ocrResult ? (
								<pre className="text-[11px] sm:text-xs leading-relaxed text-slate-200 whitespace-pre-wrap">
									{JSON.stringify(ocrResult, null, 2)}
								</pre>
							) : (
								<div className="h-full flex flex-col items-center justify-center text-slate-500 py-16 text-center">
									<p>No document submitted yet.</p>
									<p className="text-[11px] text-slate-600 mt-1">
										Upload a {docType === "sim" ? "SIM" : "KTP"} image or click
										"Load Synthetic Fixture" above.
									</p>
								</div>
							)}
						</div>
					</div>
				</div>

				{/* Extracted Structured Field Inspector */}
				{ocrResult?.data && (
					<div className="px-4 sm:px-6 py-4 bg-slate-50/80 border-t border-slate-100">
						<h4 className="text-xs font-bold text-slate-700 uppercase tracking-wider mb-3">
							Normalized Field Verification (
							{"nomor_sim" in ocrResult.data ? "SIM" : "KTP"})
						</h4>
						{"nomor_sim" in ocrResult.data ? (
							<div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-2.5 sm:gap-3 text-xs">
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Nomor SIM
									</span>
									<span className="font-mono font-bold text-slate-900">
										{ocrResult.data.nomor_sim}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Golongan
									</span>
									<span className="font-semibold text-slate-900">
										{ocrResult.data.golongan}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Nama
									</span>
									<span className="font-semibold text-slate-900 truncate block">
										{ocrResult.data.nama}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Tanggal Lahir
									</span>
									<span className="font-mono text-slate-900">
										{ocrResult.data.tanggal_lahir}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Jenis Kelamin
									</span>
									<span className="text-slate-900">
										{ocrResult.data.jenis_kelamin}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Golongan Darah
									</span>
									<span className="text-slate-900">
										{ocrResult.data.golongan_darah}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200 sm:col-span-2">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Alamat
									</span>
									<span className="text-slate-900 truncate block">
										{ocrResult.data.alamat}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Pekerjaan
									</span>
									<span className="text-slate-900">
										{ocrResult.data.pekerjaan}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Polda
									</span>
									<span className="text-slate-900">{ocrResult.data.polda}</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Masa Berlaku
									</span>
									<span className="font-mono text-emerald-600 font-semibold">
										{ocrResult.data.masa_berlaku}
									</span>
								</div>
							</div>
						) : (
							<div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-2.5 sm:gap-3 text-xs">
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										NIK
									</span>
									<span className="font-mono font-bold text-slate-900">
										{ocrResult.data.nik}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Nama
									</span>
									<span className="font-semibold text-slate-900 truncate block">
										{ocrResult.data.nama}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Tanggal Lahir
									</span>
									<span className="font-mono text-slate-900">
										{ocrResult.data.tanggal_lahir}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Jenis Kelamin
									</span>
									<span className="text-slate-900">
										{ocrResult.data.jenis_kelamin}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200 sm:col-span-2">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Alamat
									</span>
									<span className="text-slate-900 truncate block">
										{ocrResult.data.alamat}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Agama
									</span>
									<span className="text-slate-900">{ocrResult.data.agama}</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200">
									<span className="text-slate-400 block text-[10px] uppercase font-semibold">
										Status Perkawinan
									</span>
									<span className="text-slate-900">
										{ocrResult.data.status_perkawinan}
									</span>
								</div>
							</div>
						)}
					</div>
				)}
			</div>
		</div>
	);
};
