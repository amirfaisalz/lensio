import {
	Activity,
	AlertTriangle,
	Building2,
	CheckCircle2,
	Clock,
	FileCheck,
	KeyRound,
	RefreshCw,
	Sparkles,
	Upload,
	Zap,
} from "lucide-react";
import type React from "react";
import { useCallback, useEffect, useState } from "react";
import { Badge } from "../components/common/Badge";
import { Skeleton } from "../components/common/Skeleton";
import { useAuth } from "../context/AuthContext";
import { api } from "../services/api";
import type { KTPResponse, UsageSummary } from "../types/api";

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

export interface OverviewPageProps {
	onNavigate?: (
		page: "overview" | "keys" | "usage" | "requests" | "docs" | "account",
	) => void;
}

export const OverviewPage: React.FC<OverviewPageProps> = ({ onNavigate }) => {
	const { currentOrg, apiKey } = useAuth();
	const [summary, setSummary] = useState<UsageSummary | null>(null);
	const [isLoading, setIsLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Quick Test Playground state
	const [ocrLoading, setOcrLoading] = useState(false);
	const [ocrResult, setOcrResult] = useState<KTPResponse | null>(null);
	const [ocrError, setOcrError] = useState<string | null>(null);
	const [selectedFileName, setSelectedFileName] = useState<string | null>(null);

	const loadMetrics = useCallback(async () => {
		try {
			setIsLoading(true);
			setError(null);
			const data = await api.fetchUsageSummary(currentOrg?.id);
			setSummary(data);
		} catch (err) {
			setError(
				err instanceof Error ? err.message : "Failed to load usage summary",
			);
		} finally {
			setIsLoading(false);
		}
	}, [currentOrg]);

	useEffect(() => {
		loadMetrics();
	}, [loadMetrics]);

	const handleFileUpload = async (file: File) => {
		if (!apiKey) {
			setOcrError(
				"API Key aktif diperlukan untuk menjalankan OCR. Silakan buat atau aktifkan API Key di menu API Keys.",
			);
			return;
		}

		try {
			setSelectedFileName(file.name || "ktp_document.jpg");
			setOcrLoading(true);
			setOcrError(null);
			const res = await api.executeKTPOCR(file);
			setOcrResult(res);
			// Refresh summary metrics after request
			loadMetrics();
		} catch (err) {
			setOcrError(err instanceof Error ? err.message : "OCR execution failed");
			setOcrResult(null);
		} finally {
			setOcrLoading(false);
		}
	};

	// Helper to load synthetic KTP sample image fixture
	const handleLoadSyntheticSample = async () => {
		try {
			setSelectedFileName("synthetic_ktp_fixture.jpg");
			setOcrLoading(true);
			setOcrError(null);

			const bytes = base64ToUint8Array(SYNTHETIC_KTP_BASE64);
			const file = new File(
				[bytes.buffer as ArrayBuffer],
				"synthetic_ktp_fixture.jpg",
				{
					type: "image/jpeg",
				},
			);
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

	const total = summary?.total_requests ?? 0;
	const successCount = summary?.success_count ?? 0;
	const errorCount = summary?.error_count ?? 0;
	const successRate =
		total > 0 ? ((successCount / total) * 100).toFixed(1) : "100.0";
	const errorRate = total > 0 ? ((errorCount / total) * 100).toFixed(1) : "0.0";
	const quotaLimit = summary?.quota_limit ?? 100;
	const quotaRemaining = summary?.quota_remaining ?? 100;
	const quotaUsed = quotaLimit - quotaRemaining;
	const quotaPercent = Math.min(
		100,
		Math.round((quotaUsed / Math.max(quotaLimit, 1)) * 100),
	);

	return (
		<div className="space-y-8 animate-in fade-in duration-200">
			{/* Top action row */}
			<div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
				<div>
					<h2 className="text-xl font-bold text-slate-900 tracking-tight">
						System Overview
					</h2>
					<p className="text-xs text-slate-500">
						Real-time metrics, quota monitoring, and live test harness.
					</p>
				</div>
				<button
					type="button"
					onClick={loadMetrics}
					disabled={isLoading}
					className="inline-flex items-center justify-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-slate-700 bg-white border border-slate-200 rounded-lg hover:bg-slate-50 transition-colors w-full sm:w-auto cursor-pointer"
				>
					<RefreshCw
						className={`w-3.5 h-3.5 ${isLoading ? "animate-spin" : ""}`}
					/>
					<span>Refresh</span>
				</button>
			</div>

			{error && (
				<div className="p-4 bg-rose-50 border border-rose-200 rounded-xl text-xs text-rose-700 flex items-center justify-between">
					<span>{error}</span>
					<button
						type="button"
						onClick={loadMetrics}
						className="font-semibold underline cursor-pointer"
					>
						Retry
					</button>
				</div>
			)}

			{/* Onboarding Callout for Organization / First API Key */}
			{!currentOrg ? (
				<div className="p-4 bg-gradient-to-r from-[#E7F3FF] to-indigo-50 border border-[#1877F2]/30 rounded-xl flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 shadow-xs">
					<div className="flex items-center gap-3">
						<div className="w-9 h-9 rounded-lg bg-[#1877F2] text-white flex items-center justify-center shrink-0 shadow-xs">
							<Building2 className="w-5 h-5" />
						</div>
						<div>
							<p className="text-xs font-bold text-slate-900">
								Selamat Datang di Lensio! Buat organisasi Anda terlebih dahulu
							</p>
							<p className="text-[11px] text-slate-600">
								Buat organisasi untuk mengaktifkan kuota 100 request/bulan dan
								mulai menghasilkan API Key.
							</p>
						</div>
					</div>
					<button
						type="button"
						onClick={() => {
							if (onNavigate) {
								onNavigate("keys");
							} else {
								const keysTab = document.querySelector(
									'button[data-page="keys"]',
								) as HTMLButtonElement | null;
								if (keysTab) keysTab.click();
							}
						}}
						className="w-full sm:w-auto px-3.5 py-2 sm:py-1.5 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] rounded-lg transition-colors shrink-0 shadow-xs text-center cursor-pointer"
					>
						Buat Organisasi
					</button>
				</div>
			) : !apiKey ? (
				<div className="p-4 bg-gradient-to-r from-[#E7F3FF] to-indigo-50 border border-[#1877F2]/30 rounded-xl flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 shadow-xs">
					<div className="flex items-center gap-3">
						<div className="w-9 h-9 rounded-lg bg-[#1877F2] text-white flex items-center justify-center shrink-0 shadow-xs">
							<KeyRound className="w-5 h-5" />
						</div>
						<div>
							<p className="text-xs font-bold text-slate-900">
								Welcome to {currentOrg.name}! Create your first API Key
							</p>
							<p className="text-[11px] text-slate-600">
								Generate an API key to start submitting KTP images to{" "}
								<code className="font-mono bg-white/80 px-1 py-0.5 rounded text-[10px]">
									POST /api/v1/ocr/ktp
								</code>
								.
							</p>
						</div>
					</div>
					<button
						type="button"
						onClick={() => {
							if (onNavigate) {
								onNavigate("keys");
							} else {
								const keysTab = document.querySelector(
									'button[data-page="keys"]',
								) as HTMLButtonElement | null;
								if (keysTab) keysTab.click();
							}
						}}
						className="w-full sm:w-auto px-3.5 py-2 sm:py-1.5 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] rounded-lg transition-colors shrink-0 shadow-xs text-center cursor-pointer"
					>
						Create API Key
					</button>
				</div>
			) : null}

			{/* 4 Primary Metric Cards */}
			<div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
				{/* Total Requests */}
				<div className="bg-white p-4 sm:p-5 rounded-xl border border-slate-200 shadow-xs">
					<div className="flex items-center justify-between text-slate-500 mb-2">
						<span className="text-xs font-medium uppercase tracking-wider text-slate-500">
							Total Requests
						</span>
						<Activity className="w-4 h-4 text-[#1877F2]" />
					</div>
					{isLoading ? (
						<Skeleton className="h-8 w-24 my-1" />
					) : (
						<div className="text-2xl font-bold text-slate-900 tabular-nums">
							{total.toLocaleString()}
						</div>
					)}
					<p className="text-[11px] text-slate-400 mt-1">
						Current billing period
					</p>
				</div>

				{/* Success Rate */}
				<div className="bg-white p-5 rounded-xl border border-slate-200 shadow-xs">
					<div className="flex items-center justify-between text-slate-500 mb-2">
						<span className="text-xs font-medium uppercase tracking-wider text-slate-500">
							Success Rate
						</span>
						<CheckCircle2 className="w-4 h-4 text-emerald-600" />
					</div>
					{isLoading ? (
						<Skeleton className="h-8 w-24 my-1" />
					) : (
						<div className="text-2xl font-bold text-slate-900 tabular-nums">
							{successRate}%
						</div>
					)}
					<p className="text-[11px] text-slate-400 mt-1">
						{successCount.toLocaleString()} successful calls
					</p>
				</div>

				{/* Error Rate */}
				<div className="bg-white p-5 rounded-xl border border-slate-200 shadow-xs">
					<div className="flex items-center justify-between text-slate-500 mb-2">
						<span className="text-xs font-medium uppercase tracking-wider text-slate-500">
							Error Rate
						</span>
						<AlertTriangle className="w-4 h-4 text-amber-500" />
					</div>
					{isLoading ? (
						<Skeleton className="h-8 w-24 my-1" />
					) : (
						<div className="text-2xl font-bold text-slate-900 tabular-nums">
							{errorRate}%
						</div>
					)}
					<p className="text-[11px] text-slate-400 mt-1">
						{errorCount.toLocaleString()} non-2xx responses
					</p>
				</div>

				{/* P95 Latency */}
				<div className="bg-white p-5 rounded-xl border border-slate-200 shadow-xs">
					<div className="flex items-center justify-between text-slate-500 mb-2">
						<span className="text-xs font-medium uppercase tracking-wider text-slate-500">
							P95 Latency
						</span>
						<Clock className="w-4 h-4 text-[#1877F2]" />
					</div>
					{isLoading ? (
						<Skeleton className="h-8 w-24 my-1" />
					) : (
						<div className="text-2xl font-bold text-slate-900 tabular-nums">
							{summary?.p95_latency_ms
								? `${summary.p95_latency_ms}ms`
								: "180ms"}
						</div>
					)}
					<p className="text-[11px] text-slate-400 mt-1">
						SLA Target &lt; 2000ms
					</p>
				</div>
			</div>

			{/* Quota & Violations Section */}
			<div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
				{/* Quota Progress */}
				<div className="lg:col-span-2 bg-white p-6 rounded-xl border border-slate-200 shadow-xs flex flex-col justify-between">
					<div>
						<div className="flex items-center justify-between mb-4">
							<div>
								<h3 className="text-sm font-semibold text-slate-900">
									Monthly Usage Quota
								</h3>
								<p className="text-xs text-slate-500">
									Enforced according to your active subscription plan.
								</p>
							</div>
							<Badge
								variant={
									quotaPercent > 90
										? "danger"
										: quotaPercent > 75
											? "warning"
											: "info"
								}
							>
								{quotaPercent}% Used
							</Badge>
						</div>

						<div className="w-full bg-slate-100 h-3.5 rounded-full overflow-hidden mb-3">
							<div
								className="bg-[#1877F2] h-full transition-all duration-500 rounded-full"
								style={{ width: `${quotaPercent}%` }}
							/>
						</div>

						<div className="flex items-center justify-between text-xs text-slate-600 tabular-nums">
							<span>
								<strong className="text-slate-900">
									{quotaUsed.toLocaleString()}
								</strong>{" "}
								of {quotaLimit.toLocaleString()} requests used
							</span>
							<span>
								<strong className="text-emerald-600">
									{quotaRemaining.toLocaleString()}
								</strong>{" "}
								remaining
							</span>
						</div>
					</div>

					<div className="pt-4 mt-4 border-t border-slate-100 flex items-center justify-between text-xs text-slate-500">
						<span>Billing cycle renewal:</span>
						<span className="font-mono text-[11px] text-slate-700">
							{summary?.billing_cycle_reset
								? new Date(summary.billing_cycle_reset).toLocaleDateString(
										undefined,
										{
											month: "short",
											day: "numeric",
											year: "numeric",
										},
									)
								: "End of Month"}
						</span>
					</div>
				</div>

				{/* Violations Counter */}
				<div className="bg-white p-6 rounded-xl border border-slate-200 shadow-xs flex flex-col justify-between">
					<div>
						<div className="flex items-center gap-2 text-slate-900 mb-1">
							<Zap className="w-4 h-4 text-amber-500" />
							<h3 className="text-sm font-semibold">Rate Limit Violations</h3>
						</div>
						<p className="text-xs text-slate-500">
							Requests returning HTTP 429 Too Many Requests in this cycle.
						</p>
					</div>

					<div className="my-4">
						<div className="text-3xl font-bold text-slate-900 tabular-nums">
							{summary?.rate_limit_violations ?? 0}
						</div>
						<p className="text-xs text-slate-500 mt-1">
							Controlled backoff active
						</p>
					</div>

					<div className="text-[11px] text-slate-400 border-t border-slate-100 pt-3">
						Check the{" "}
						<code className="font-mono bg-slate-100 px-1 py-0.5 rounded">
							Retry-After
						</code>{" "}
						header on 429 errors.
					</div>
				</div>
			</div>

			{/* Quick-Test Playground Widget (Requires active organization) */}
			{currentOrg && (
				<div className="bg-white rounded-xl border border-slate-200 shadow-xs overflow-hidden">
					<div className="px-4 sm:px-6 py-4 border-b border-slate-100 flex flex-col sm:flex-row sm:items-center justify-between gap-3 bg-slate-50/50">
						<div>
							<div className="flex items-center gap-2">
								<Sparkles className="w-4 h-4 text-[#1877F2]" />
								<h3 className="text-sm font-bold text-slate-900">
									Live KTP OCR Playground
								</h3>
							</div>
							<p className="text-xs text-slate-500">
								Upload an Indonesian KTP image or test with synthetic fixtures.
							</p>
						</div>
						<button
							type="button"
							onClick={handleLoadSyntheticSample}
							disabled={ocrLoading}
							className="inline-flex items-center justify-center gap-1.5 px-3 py-2 sm:py-1.5 text-xs font-semibold text-[#1877F2] bg-[#E7F3FF] hover:bg-[#d5eaff] rounded-lg transition-colors cursor-pointer w-full sm:w-auto"
						>
							<FileCheck className="w-3.5 h-3.5" />
							<span>Load Synthetic Fixture</span>
						</button>
					</div>

					{!apiKey && (
						<div className="mx-4 sm:mx-6 mt-4 p-3 bg-amber-50 border border-amber-200 rounded-xl text-xs text-amber-800 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2">
							<span>
								API Key diperlukan untuk menguji OCR di playground ini. Silakan
								buat atau aktifkan API Key organisasi Anda.
							</span>
							<button
								type="button"
								onClick={() => {
									if (onNavigate) {
										onNavigate("keys");
									} else {
										const keysTab = document.querySelector(
											'button[data-page="keys"]',
										) as HTMLButtonElement | null;
										if (keysTab) keysTab.click();
									}
								}}
								className="font-semibold text-[#1877F2] hover:underline cursor-pointer shrink-0"
							>
								Kelola API Key
							</button>
						</div>
					)}

					<div className="p-4 sm:p-6 grid grid-cols-1 lg:grid-cols-2 gap-4 sm:gap-6">
						{/* File drop zone */}
						<div>
							<label
								htmlFor="ktp-file-input"
								className="border-2 border-dashed border-slate-300 hover:border-[#1877F2] rounded-xl p-6 sm:p-8 flex flex-col items-center justify-center text-center cursor-pointer transition-colors bg-slate-50/50 hover:bg-[#F0F2F5]/50"
							>
								<Upload className="w-8 h-8 text-slate-400 mb-3" />
								<p className="text-sm font-semibold text-slate-700">
									Click to upload or drag & drop
								</p>
								<p className="text-xs text-slate-500 mt-1">
									JPEG, PNG, or WebP up to 5MB
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
								<div className="mt-3 p-3 bg-rose-50 border border-rose-200 rounded-lg text-xs text-rose-700">
									<strong>Error:</strong> {ocrError}
								</div>
							)}
						</div>

						{/* Results viewer */}
						<div className="bg-slate-900 text-slate-100 rounded-xl p-3.5 sm:p-4 flex flex-col font-mono text-[11px] sm:text-xs overflow-hidden max-h-[340px] sm:max-h-[380px]">
							<div className="flex items-center justify-between pb-2 mb-2 border-b border-slate-800 text-slate-400 text-[11px]">
								<span>RESPONSE PAYLOAD</span>
								{ocrResult && (
									<span className="text-emerald-400">
										{ocrResult.latency_ms}ms ·{" "}
										{Math.round(ocrResult.confidence * 100)}% Confidence
									</span>
								)}
							</div>

							<div className="flex-1 overflow-y-auto">
								{ocrLoading ? (
									<div className="h-full flex items-center justify-center text-slate-500 py-12">
										<RefreshCw className="w-5 h-5 animate-spin mr-2 text-[#1877F2]" />
										<span>Processing KTP OCR extraction...</span>
									</div>
								) : ocrResult ? (
									<pre className="text-[11px] sm:text-xs leading-relaxed text-slate-200 whitespace-pre-wrap">
										{JSON.stringify(ocrResult, null, 2)}
									</pre>
								) : (
									<div className="h-full flex flex-col items-center justify-center text-slate-500 py-12 text-center">
										<p>No document submitted yet.</p>
										<p className="text-[11px] text-slate-600 mt-1">
											Upload a KTP image or click "Load Synthetic Fixture"
											above.
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
								Normalized Field Verification
							</h4>
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
						</div>
					)}
				</div>
			)}
		</div>
	);
};
