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
import type { KTPResponse, NPWPResponse, PassportResponse, SIMResponse } from "../types/api";

// 400x250 valid synthetic KTP JPEG fixture
const SYNTHETIC_KTP_BASE64 =
	"/9j/2wCEAAMCAgMCAgMDAwMEAwMEBQgFBQQEBQoHBwYIDAoMDAsKCwsNDhIQDQ4RDgsLEBYQERMUFRUVDA8XGBYUGBIUFRQBAwQEBQQFCQUFCRQNCw0UFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFP/AABEIAPoBkAMBIgACEQEDEQH/xAGiAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgsQAAIBAwMCBAMFBQQEAAABfQECAwAEEQUSITFBBhNRYQcicRQygZGhCCNCscEVUtHwJDNicoIJChYXGBkaJSYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6g4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2drh4uPk5ebn6Onq8fLz9PX29/j5+gEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoLEQACAQIEBAMEBwUEBAABAncAAQIDEQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEKFiQ04SXxFxgZGiYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6g4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2dri4+Tl5ufo6ery8/T19vf4+fr/2gAMAwEAAhEDEQA/AP0Fooor6M+fCiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAK//Z";

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

	const [docType, setDocType] = useState<"ktp" | "sim" | "passport" | "npwp">("ktp");
	const [ocrLoading, setOcrLoading] = useState(false);
	const [ocrResult, setOcrResult] = useState<
		KTPResponse | SIMResponse | PassportResponse | NPWPResponse | null
	>(null);
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
					(docType === "npwp"
						? "npwp_document.jpg"
						: docType === "passport"
							? "passport_document.jpg"
							: docType === "sim"
								? "sim_document.jpg"
								: "ktp_document.jpg"),
			);
			setOcrLoading(true);
			setOcrError(null);
			let res: KTPResponse | SIMResponse | PassportResponse | NPWPResponse;
			if (docType === "npwp") {
				res = await api.executeNPWPOCR(file, apiKey);
			} else if (docType === "passport") {
				res = await api.executePassportOCR(file, apiKey);
			} else if (docType === "sim") {
				res = await api.executeSIMOCR(file, apiKey);
			} else {
				res = await api.executeKTPOCR(file, apiKey);
			}
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
				docType === "npwp"
					? "synthetic_npwp_fixture.jpg"
					: docType === "passport"
						? "synthetic_passport_fixture.jpg"
						: docType === "sim"
							? "synthetic_sim_fixture.jpg"
							: "synthetic_ktp_fixture.jpg";
			setSelectedFileName(filename);
			setOcrLoading(true);
			setOcrError(null);

			let bytes = base64ToUint8Array(SYNTHETIC_KTP_BASE64);
			if (docType === "npwp") {
				const marker = new TextEncoder().encode("MOCK_NPWP_DOC");
				const combined = new Uint8Array(bytes.length + marker.length);
				combined.set(bytes);
				combined.set(marker, bytes.length);
				bytes = combined;
			} else if (docType === "passport") {
				const marker = new TextEncoder().encode("MOCK_PASSPORT_DOC");
				const combined = new Uint8Array(bytes.length + marker.length);
				combined.set(bytes);
				combined.set(marker, bytes.length);
				bytes = combined;
			} else if (docType === "sim") {
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
			<div className="space-y-6">
				<div>
					<h2 className="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
						Live OCR Playground
					</h2>
					<p className="text-xs text-slate-500 dark:text-slate-400">
						Interactive test harness for Indonesian identity documents.
					</p>
				</div>

				<div className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-white/10 p-8 text-center max-w-lg mx-auto">
					<div className="w-12 h-12 rounded-xl bg-[#E7F3FF] dark:bg-[#1877F2]/20 text-[#1877F2] dark:text-[#7aa9f5] flex items-center justify-center mx-auto mb-4">
						<Building2 className="w-6 h-6" />
					</div>
					<h3 className="text-base font-bold text-slate-900 dark:text-white mb-1">
						Pilih atau Buat Organisasi
					</h3>
					<p className="text-xs text-slate-500 dark:text-slate-400 mb-6 leading-relaxed">
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
		<div className="space-y-6">
			{/* Page Header */}
			<div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
				<div>
					<h2 className="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
						Live OCR Playground
					</h2>
					<p className="text-xs text-slate-500 dark:text-slate-400">
						Interactive test harness for Indonesian identity documents (KTP, SIM, Passport & NPWP)
						with synthetic fixtures.
					</p>
				</div>

				{/* Document Switcher & Fixture Button */}
				<div className="flex items-center gap-2.5">
					<div className="flex items-center bg-slate-200/70 dark:bg-white/10 p-0.5 rounded-lg text-xs font-semibold">
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
									? "bg-white dark:bg-slate-700 text-[#1877F2] dark:text-white"
									: "text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:text-white dark:hover:text-white"
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
									? "bg-white dark:bg-slate-700 text-[#1877F2] dark:text-white"
									: "text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:text-white dark:hover:text-white"
							}`}
						>
							SIM
						</button>
						<button
							type="button"
							onClick={() => {
								setDocType("passport");
								setOcrResult(null);
								setSelectedFileName(null);
								setOcrError(null);
							}}
							className={`px-3 py-1 rounded-md transition-all cursor-pointer ${
								docType === "passport"
									? "bg-white dark:bg-slate-700 text-[#1877F2] dark:text-white"
									: "text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:text-white dark:hover:text-white"
							}`}
						>
							Passport
						</button>
						<button
							type="button"
							onClick={() => {
								setDocType("npwp");
								setOcrResult(null);
								setSelectedFileName(null);
								setOcrError(null);
							}}
							className={`px-3 py-1 rounded-md transition-all cursor-pointer ${
								docType === "npwp"
									? "bg-white dark:bg-slate-700 text-[#1877F2] dark:text-white"
									: "text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:text-white dark:hover:text-white"
							}`}
						>
							NPWP
						</button>
					</div>
					<button
						type="button"
						onClick={handleLoadSyntheticSample}
						disabled={ocrLoading}
						className="inline-flex items-center justify-center gap-1.5 px-3.5 py-1.5 text-xs font-semibold text-[#1877F2] dark:text-[#7aa9f5] bg-[#E7F3FF] dark:bg-[#1877F2]/20 hover:bg-[#d5eaff] dark:hover:bg-[#1877F2]/30 rounded-lg transition-colors cursor-pointer"
					>
						<FileCheck className="w-3.5 h-3.5" />
						<span>Load Synthetic Fixture</span>
					</button>
				</div>
			</div>

			{/* Missing API Key Warning */}
			{!apiKey && (
				<div className="p-4 bg-amber-50 dark:bg-amber-500/10 border border-amber-200 dark:border-amber-500/20 rounded-xl text-xs text-amber-800 dark:text-amber-300 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
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
			<div className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-white/10 overflow-hidden">
				<div className="p-4 sm:p-6 grid grid-cols-1 lg:grid-cols-2 gap-4 sm:gap-6">
					{/* Left: File Drop Zone */}
					<div>
						<label
							htmlFor="ktp-file-input"
							className="border-2 border-dashed border-slate-300 dark:border-white/20 hover:border-[#1877F2] rounded-xl p-8 flex flex-col items-center justify-center text-center cursor-pointer transition-colors bg-slate-50/50 dark:bg-white/5 hover:bg-[#F0F2F5]/50 dark:hover:bg-white/10 min-h-[220px]"
						>
							<Upload className="w-8 h-8 text-slate-400 dark:text-slate-500 mb-3" />
							<p className="text-sm font-semibold text-slate-700 dark:text-slate-300">
								Click to upload or drag & drop
							</p>
							<p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
								Upload Indonesian {docType === "passport" ? "Passport" : docType === "npwp" ? "NPWP" : docType === "sim" ? "SIM" : "KTP"} image
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
							<div className="mt-3 flex items-center justify-between px-3 py-2 bg-slate-50 dark:bg-white/5 rounded-lg border border-slate-200 dark:border-white/10 text-xs text-slate-600 dark:text-slate-400">
								<span className="truncate font-mono">{selectedFileName}</span>
								{ocrLoading && (
									<RefreshCw className="w-3.5 h-3.5 animate-spin text-[#1877F2]" />
								)}
							</div>
						)}

						{ocrError && (
							<div className="mt-3 p-3 bg-rose-50 dark:bg-rose-500/10 border border-rose-200 dark:border-rose-500/20 rounded-lg text-xs text-rose-700 dark:text-rose-300 flex items-start gap-2">
								<AlertTriangle className="w-4 h-4 text-rose-500 shrink-0 mt-0.5" />
								<span>
									<strong>Error:</strong> {ocrError}
								</span>
							</div>
						)}
					</div>

					{/* Right: Results JSON Viewer */}
					<div className="bg-slate-900 text-slate-100 rounded-xl p-4 flex flex-col font-mono text-[11px] sm:text-xs overflow-hidden min-h-[260px] max-h-[420px]">
						<div className="flex items-center justify-between pb-2 mb-2 border-b border-slate-800 text-slate-400 dark:text-slate-500 text-[11px]">
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
								<div className="h-full flex items-center justify-center text-slate-500 dark:text-slate-400 py-16">
									<RefreshCw className="w-5 h-5 animate-spin mr-2 text-[#1877F2]" />
									<span>
										Processing {docType === "passport" ? "Passport" : docType === "npwp" ? "NPWP" : docType === "sim" ? "SIM" : "KTP"} OCR
										extraction...
									</span>
								</div>
							) : ocrResult ? (
								<pre className="text-[11px] sm:text-xs leading-relaxed text-slate-200 whitespace-pre-wrap">
									{JSON.stringify(ocrResult, null, 2)}
								</pre>
							) : (
								<div className="h-full flex flex-col items-center justify-center text-slate-500 dark:text-slate-400 py-16 text-center">
									<p>No document submitted yet.</p>
									<p className="text-[11px] text-slate-600 dark:text-slate-400 mt-1">
										Upload a {docType === "passport" ? "Passport" : docType === "npwp" ? "NPWP" : docType === "sim" ? "SIM" : "KTP"} image or click
										"Load Synthetic Fixture" above.
									</p>
								</div>
							)}
						</div>
					</div>
				</div>

				{/* Extracted Structured Field Inspector */}
				{ocrResult?.data && (
					<div className="px-4 sm:px-6 py-4 bg-slate-50/80 dark:bg-white/[0.02] border-t border-slate-100 dark:border-white/10">
						<h4 className="text-xs font-bold text-slate-700 dark:text-slate-300 uppercase tracking-wider mb-3">
							Normalized Field Verification (
							{"passport_number" in ocrResult.data
								? "Passport"
								: "nomor_sim" in ocrResult.data
									? "SIM"
									: "npwp" in ocrResult.data
										? "NPWP"
										: "KTP"}
							)
						</h4>
						{"passport_number" in ocrResult.data ? (
							<div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-2.5 sm:gap-3 text-xs">
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Nomor Paspor
									</span>
									<span className="font-mono font-bold text-slate-900 dark:text-white">
										{ocrResult.data.passport_number}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Nama Lengkap
									</span>
									<span className="font-semibold text-slate-900 dark:text-white truncate block">
										{ocrResult.data.full_name}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Kewarganegaraan
									</span>
									<span className="font-semibold text-slate-900 dark:text-white">
										{ocrResult.data.nationality}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Tempat Lahir
									</span>
									<span className="text-slate-900 dark:text-white">
										{ocrResult.data.place_of_birth}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Tanggal Lahir
									</span>
									<span className="font-mono text-slate-900 dark:text-white">
										{ocrResult.data.date_of_birth}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Jenis Kelamin
									</span>
									<span className="text-slate-900 dark:text-white">
										{ocrResult.data.gender}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Tanggal Pengeluaran
									</span>
									<span className="font-mono text-slate-900 dark:text-white">
										{ocrResult.data.issue_date}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Tanggal Habis Berlaku
									</span>
									<span className="font-mono text-emerald-600 font-semibold">
										{ocrResult.data.expiry_date}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10 sm:col-span-2">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Kantor yang Mengeluarkan
									</span>
									<span className="text-slate-900 dark:text-white">
										{ocrResult.data.issuing_office}
									</span>
								</div>
								{ocrResult.data.mrz_line1 && (
									<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10 sm:col-span-2">
										<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
											MRZ Line 1
										</span>
										<span className="font-mono text-[11px] text-slate-900 dark:text-white break-all">
											{ocrResult.data.mrz_line1}
										</span>
									</div>
								)}
								{ocrResult.data.mrz_line2 && (
									<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10 sm:col-span-2">
										<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
											MRZ Line 2
										</span>
										<span className="font-mono text-[11px] text-slate-900 dark:text-white break-all">
											{ocrResult.data.mrz_line2}
										</span>
									</div>
								)}
							</div>
						) : "npwp" in ocrResult.data ? (
							<div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-2.5 sm:gap-3 text-xs">
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Nomor NPWP
									</span>
									<span className="font-mono font-bold text-slate-900 dark:text-white">
										{ocrResult.data.npwp}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Nama Wajib Pajak
									</span>
									<span className="font-semibold text-slate-900 dark:text-white truncate block">
										{ocrResult.data.nama}
									</span>
								</div>
								{ocrResult.data.nik && (
									<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
										<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
											NIK
										</span>
										<span className="font-mono text-slate-900 dark:text-white">
											{ocrResult.data.nik}
										</span>
									</div>
								)}
								{ocrResult.data.kpp && (
									<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
										<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
											Kantor Pelayanan Pajak (KPP)
										</span>
										<span className="text-slate-900 dark:text-white truncate block">
											{ocrResult.data.kpp}
										</span>
									</div>
								)}
								{ocrResult.data.alamat && (
									<div className="bg-white p-2.5 rounded border border-slate-200 sm:col-span-2">
										<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
											Alamat
										</span>
										<span className="text-slate-900 dark:text-white truncate block">
											{ocrResult.data.alamat}
										</span>
									</div>
								)}
								{ocrResult.data.tanggal_daftar && (
									<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
										<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
											Tanggal Terdaftar
										</span>
										<span className="font-mono text-emerald-600 font-semibold">
											{ocrResult.data.tanggal_daftar}
										</span>
									</div>
								)}
							</div>
						) : "nomor_sim" in ocrResult.data ? (
							<div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-2.5 sm:gap-3 text-xs">
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Nomor SIM
									</span>
									<span className="font-mono font-bold text-slate-900 dark:text-white">
										{ocrResult.data.nomor_sim}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Golongan
									</span>
									<span className="font-semibold text-slate-900 dark:text-white">
										{ocrResult.data.golongan}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Nama
									</span>
									<span className="font-semibold text-slate-900 dark:text-white truncate block">
										{ocrResult.data.nama}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Tanggal Lahir
									</span>
									<span className="font-mono text-slate-900 dark:text-white">
										{ocrResult.data.tanggal_lahir}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Jenis Kelamin
									</span>
									<span className="text-slate-900 dark:text-white">
										{ocrResult.data.jenis_kelamin}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Golongan Darah
									</span>
									<span className="text-slate-900 dark:text-white">
										{ocrResult.data.golongan_darah}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200 sm:col-span-2">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Alamat
									</span>
									<span className="text-slate-900 dark:text-white truncate block">
										{ocrResult.data.alamat}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Pekerjaan
									</span>
									<span className="text-slate-900 dark:text-white">
										{ocrResult.data.pekerjaan}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Polda
									</span>
									<span className="text-slate-900 dark:text-white">
										{ocrResult.data.polda}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Masa Berlaku
									</span>
									<span className="font-mono text-emerald-600 font-semibold">
										{ocrResult.data.masa_berlaku}
									</span>
								</div>
							</div>
						) : (
							<div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-2.5 sm:gap-3 text-xs">
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										NIK
									</span>
									<span className="font-mono font-bold text-slate-900 dark:text-white">
										{ocrResult.data.nik}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Nama
									</span>
									<span className="font-semibold text-slate-900 dark:text-white truncate block">
										{ocrResult.data.nama}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Tanggal Lahir
									</span>
									<span className="font-mono text-slate-900 dark:text-white">
										{ocrResult.data.tanggal_lahir}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Jenis Kelamin
									</span>
									<span className="text-slate-900 dark:text-white">
										{ocrResult.data.jenis_kelamin}
									</span>
								</div>
								<div className="bg-white p-2.5 rounded border border-slate-200 sm:col-span-2">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Alamat
									</span>
									<span className="text-slate-900 dark:text-white truncate block">
										{ocrResult.data.alamat}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Agama
									</span>
									<span className="text-slate-900 dark:text-white">
										{ocrResult.data.agama}
									</span>
								</div>
								<div className="bg-white dark:bg-slate-800 p-2.5 rounded border border-slate-200 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Status Perkawinan
									</span>
									<span className="text-slate-900 dark:text-white">
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
