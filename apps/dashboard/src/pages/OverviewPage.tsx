import {
	Activity,
	AlertTriangle,
	ArrowRight,
	BookOpen,
	Building2,
	CheckCircle2,
	Clock,
	KeyRound,
	RefreshCw,
	Sparkles,
	Timer,
	Zap,
} from "lucide-react";
import type React from "react";
import { useCallback, useEffect, useState } from "react";
import { Badge } from "../components/common/Badge";
import { Skeleton } from "../components/common/Skeleton";
import type { NavigationPage } from "../components/layout/Sidebar";
import { useAuth } from "../context/AuthContext";
import { api } from "../services/api";
import type { UsageSummary } from "../types/api";

export interface OverviewPageProps {
	onNavigate?: (page: NavigationPage) => void;
}

export const OverviewPage: React.FC<OverviewPageProps> = ({ onNavigate }) => {
	const { currentOrg, apiKey } = useAuth();
	const [summary, setSummary] = useState<UsageSummary | null>(null);
	const [hasKeys, setHasKeys] = useState<boolean | null>(null);
	const [isLoading, setIsLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

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
		if (!currentOrg?.id) {
			setHasKeys(null);
			return;
		}
		try {
			const keys = await api.fetchAPIKeys(currentOrg.id);
			setHasKeys(keys.length > 0);
		} catch {
			setHasKeys(null);
		}
	}, [currentOrg]);

	useEffect(() => {
		loadMetrics();
	}, [loadMetrics]);

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
		<div className="space-y-6">
			{/* Top action row */}
			<div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
				<div>
					<h2 className="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
						System Overview
					</h2>
					<p className="text-xs text-slate-500 dark:text-slate-400">
						Real-time metrics, quota monitoring, and live test harness.
					</p>
				</div>
				<button
					type="button"
					onClick={loadMetrics}
					disabled={isLoading}
					className="inline-flex items-center justify-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-300 bg-white dark:bg-white/5 border border-slate-200 dark:border-white/10 rounded-lg hover:bg-slate-50 dark:hover:bg-white/10 transition-colors w-full sm:w-auto cursor-pointer"
				>
					<RefreshCw
						className={`w-3.5 h-3.5 ${isLoading ? "animate-spin" : ""}`}
					/>
					<span>Refresh</span>
				</button>
			</div>

			{error && (
				<div className="p-4 bg-rose-50 dark:bg-rose-500/10 border border-rose-200 dark:border-rose-500/20 rounded-xl text-xs text-rose-700 dark:text-rose-300 flex items-center justify-between">
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
				<div className="p-4 bg-[#E7F3FF]/60 dark:bg-[#1877F2]/10 border border-[#1877F2]/25 dark:border-[#1877F2]/30 rounded-xl flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
					<div className="flex items-center gap-3">
						<div className="w-9 h-9 rounded-lg bg-[#1877F2] text-white flex items-center justify-center shrink-0">
							<Building2 className="w-5 h-5" />
						</div>
						<div>
							<p className="text-xs font-bold text-slate-900 dark:text-white">
								Selamat Datang di Lensio! Buat organisasi Anda terlebih dahulu
							</p>
							<p className="text-[11px] text-slate-600 dark:text-slate-400">
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
						className="w-full sm:w-auto px-3.5 py-2 sm:py-1.5 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] rounded-lg transition-colors shrink-0 text-center cursor-pointer"
					>
						Buat Organisasi
					</button>
				</div>
			) : !apiKey && hasKeys === false ? (
				<div className="p-4 bg-[#E7F3FF]/60 dark:bg-[#1877F2]/10 border border-[#1877F2]/25 dark:border-[#1877F2]/30 rounded-xl flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
					<div className="flex items-center gap-3">
						<div className="w-9 h-9 rounded-lg bg-[#1877F2] text-white flex items-center justify-center shrink-0">
							<KeyRound className="w-5 h-5" />
						</div>
						<div>
							<p className="text-xs font-bold text-slate-900 dark:text-white">
								Welcome to {currentOrg.name}! Create your first API Key
							</p>
							<p className="text-[11px] text-slate-600 dark:text-slate-400">
								Generate an API key to start submitting identity documents to{" "}
								<code className="font-mono bg-white/80 dark:bg-white/10 px-1 py-0.5 rounded text-[10px]">
									POST /api/v1/ocr/ktp
								</code>{" "}
								or{" "}
								<code className="font-mono bg-white/80 dark:bg-white/10 px-1 py-0.5 rounded text-[10px]">
									POST /api/v1/ocr/sim
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
						className="w-full sm:w-auto px-3.5 py-2 sm:py-1.5 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] rounded-lg transition-colors shrink-0 text-center cursor-pointer"
					>
						Create API Key
					</button>
				</div>
			) : null}

			{/* 4 Primary Metric Cards */}
			<div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
				{/* Total Requests */}
				<div className="bg-white dark:bg-slate-900 p-4 sm:p-5 rounded-xl border border-slate-200 dark:border-white/10">
					<div className="flex items-center justify-between text-slate-500 dark:text-slate-400 mb-2">
						<span className="text-xs font-medium uppercase tracking-wider text-slate-500 dark:text-slate-400">
							Total Requests
						</span>
						<Activity className="w-4 h-4 text-[#1877F2]" />
					</div>
					{isLoading ? (
						<Skeleton className="h-8 w-24 my-1" />
					) : (
						<div className="text-2xl font-bold text-slate-900 dark:text-white tabular-nums">
							{total.toLocaleString()}
						</div>
					)}
					<p className="text-[11px] text-slate-400 dark:text-slate-500 mt-1">
						Current billing period
					</p>
				</div>

				{/* Success Rate */}
				<div className="bg-white dark:bg-slate-900 p-5 rounded-xl border border-slate-200 dark:border-white/10">
					<div className="flex items-center justify-between text-slate-500 dark:text-slate-400 mb-2">
						<span className="text-xs font-medium uppercase tracking-wider text-slate-500 dark:text-slate-400">
							Success Rate
						</span>
						<CheckCircle2 className="w-4 h-4 text-emerald-600" />
					</div>
					{isLoading ? (
						<Skeleton className="h-8 w-24 my-1" />
					) : (
						<div className="text-2xl font-bold text-slate-900 dark:text-white tabular-nums">
							{successRate}%
						</div>
					)}
					<p className="text-[11px] text-slate-400 dark:text-slate-500 mt-1">
						{successCount.toLocaleString()} successful calls
					</p>
				</div>

				{/* Error Rate */}
				<div className="bg-white dark:bg-slate-900 p-5 rounded-xl border border-slate-200 dark:border-white/10">
					<div className="flex items-center justify-between text-slate-500 dark:text-slate-400 mb-2">
						<span className="text-xs font-medium uppercase tracking-wider text-slate-500 dark:text-slate-400">
							Error Rate
						</span>
						<AlertTriangle className="w-4 h-4 text-amber-500" />
					</div>
					{isLoading ? (
						<Skeleton className="h-8 w-24 my-1" />
					) : (
						<div className="text-2xl font-bold text-slate-900 dark:text-white tabular-nums">
							{errorRate}%
						</div>
					)}
					<p className="text-[11px] text-slate-400 dark:text-slate-500 mt-1">
						{errorCount.toLocaleString()} non-2xx responses
					</p>
				</div>

				{/* P95 Latency */}
				<div className="bg-white dark:bg-slate-900 p-5 rounded-xl border border-slate-200 dark:border-white/10">
					<div className="flex items-center justify-between text-slate-500 dark:text-slate-400 mb-2">
						<span className="text-xs font-medium uppercase tracking-wider text-slate-500 dark:text-slate-400">
							P95 Latency
						</span>
						<Clock className="w-4 h-4 text-[#1877F2]" />
					</div>
					{isLoading ? (
						<Skeleton className="h-8 w-24 my-1" />
					) : (
						<div className="text-2xl font-bold text-slate-900 dark:text-white tabular-nums">
							{summary?.p95_latency_ms
								? `${summary.p95_latency_ms}ms`
								: "180ms"}
						</div>
					)}
					<p className="text-[11px] text-slate-400 dark:text-slate-500 mt-1">
						SLA Target &lt; 2000ms
					</p>
				</div>
			</div>

			{/* Quota, Violations & Upstream AI Section */}
			<div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
				{/* Quota Progress */}
				<div className="lg:col-span-2 bg-white dark:bg-slate-900 p-6 rounded-xl border border-slate-200 dark:border-white/10 flex flex-col justify-between">
					<div>
						<div className="flex items-center justify-between mb-4">
							<div>
								<h3 className="text-sm font-semibold text-slate-900 dark:text-white">
									Monthly Usage Quota
								</h3>
								<p className="text-xs text-slate-500 dark:text-slate-400">
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

						<div className="w-full bg-slate-100 dark:bg-white/10 h-2.5 rounded-full overflow-hidden mb-3">
							<div
								className="bg-[#1877F2] h-full transition-all duration-500 rounded-full"
								style={{ width: `${quotaPercent}%` }}
							/>
						</div>

						<div className="flex items-center justify-between text-xs text-slate-600 dark:text-slate-400 tabular-nums">
							<span>
								<strong className="text-slate-900 dark:text-white">
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

					<div className="pt-4 mt-4 border-t border-slate-100 dark:border-white/10 flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
						<span>Billing cycle renewal:</span>
						<span className="font-mono text-[11px] text-slate-700 dark:text-slate-300">
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
				<div className="bg-white dark:bg-slate-900 p-6 rounded-xl border border-slate-200 dark:border-white/10 flex flex-col justify-between">
					<div>
						<div className="flex items-center gap-2 text-slate-900 dark:text-white mb-1">
							<Zap className="w-4 h-4 text-amber-500" />
							<h3 className="text-sm font-semibold">Rate Limit Violations</h3>
						</div>
						<p className="text-xs text-slate-500 dark:text-slate-400">
							Requests returning HTTP 429 Too Many Requests in this cycle.
						</p>
					</div>

					<div className="my-4">
						<div className="text-3xl font-bold text-slate-900 dark:text-white tabular-nums">
							{summary?.rate_limit_violations ?? 0}
						</div>
						<p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
							Controlled backoff active
						</p>
					</div>

					<div className="text-[11px] text-slate-400 dark:text-slate-500 border-t border-slate-100 pt-3">
						Check the{" "}
						<code className="font-mono bg-slate-100 px-1 py-0.5 rounded">
							Retry-After
						</code>{" "}
						header on 429 errors.
					</div>
				</div>

				{/* Upstream AI Latency */}
				<div className="bg-white dark:bg-slate-900 p-6 rounded-xl border border-slate-200 dark:border-white/10 flex flex-col justify-between">
					<div>
						<div className="flex items-center gap-2 text-slate-900 dark:text-white mb-1">
							<Timer className="w-4 h-4 text-violet-500" />
							<h3 className="text-sm font-semibold">Upstream AI Latency</h3>
						</div>
						<p className="text-xs text-slate-500 dark:text-slate-400">
							P95 Vision AI time across OCR endpoints in this cycle.
						</p>
					</div>

					<div className="my-4">
						<div className="text-3xl font-bold text-slate-900 dark:text-white tabular-nums">
							{summary?.p95_ocr_latency_ms
								? `${summary.p95_ocr_latency_ms.toLocaleString()}ms`
								: "—"}
						</div>
						<p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
							No SLA · vendor-dependent
						</p>
					</div>

					<div className="text-[11px] text-slate-400 dark:text-slate-500 border-t border-slate-100 pt-3">
						Excluded from the platform P95 SLA above.
					</div>
				</div>
			</div>

			{/* Quick Action Navigation Cards */}
			<div className="grid grid-cols-1 md:grid-cols-3 gap-4 sm:gap-6">
				{/* Card 1: OCR Playground */}
				<div className="bg-white dark:bg-slate-900 p-5 sm:p-6 rounded-xl border border-slate-200 dark:border-white/10 flex flex-col justify-between hover:border-[#1877F2]/40 transition-colors group">
					<div>
						<div className="w-10 h-10 rounded-xl bg-[#E7F3FF] text-[#1877F2] flex items-center justify-center mb-4 group-hover:scale-105 transition-transform">
							<Sparkles className="w-5 h-5" />
						</div>
						<h3 className="text-sm font-bold text-slate-900 dark:text-white mb-1">
							Live OCR Playground
						</h3>
						<p className="text-xs text-slate-500 dark:text-slate-400 leading-relaxed mb-4">
							Uji ekstraksi dokumen (KTP, SIM, Passport, NPWP, KK & Invoice)
							secara langsung dengan synthetic fixture atau unggahan gambar.
						</p>
					</div>
					<button
						type="button"
						onClick={() => onNavigate?.("playground")}
						className="inline-flex items-center gap-1.5 text-xs font-semibold text-[#1877F2] hover:text-[#166FE5] group-hover:translate-x-0.5 transition-all cursor-pointer text-left"
					>
						<span>Buka Playground</span>
						<ArrowRight className="w-3.5 h-3.5" />
					</button>
				</div>

				{/* Card 2: API Keys */}
				<div className="bg-white dark:bg-slate-900 p-5 sm:p-6 rounded-xl border border-slate-200 dark:border-white/10 flex flex-col justify-between hover:border-[#1877F2]/40 transition-colors group">
					<div>
						<div className="w-10 h-10 rounded-xl bg-indigo-50 text-indigo-600 flex items-center justify-center mb-4 group-hover:scale-105 transition-transform">
							<KeyRound className="w-5 h-5" />
						</div>
						<h3 className="text-sm font-bold text-slate-900 dark:text-white mb-1">
							Kelola API Key
						</h3>
						<p className="text-xs text-slate-500 dark:text-slate-400 leading-relaxed mb-4">
							Buat token API dengan hash SHA-256 dan izin bertingkat (scope)
							untuk integrasi aplikasi produksi.
						</p>
					</div>
					<button
						type="button"
						onClick={() => onNavigate?.("keys")}
						className="inline-flex items-center gap-1.5 text-xs font-semibold text-indigo-600 hover:text-indigo-700 group-hover:translate-x-0.5 transition-all cursor-pointer text-left"
					>
						<span>Atur Kunci API</span>
						<ArrowRight className="w-3.5 h-3.5" />
					</button>
				</div>

				{/* Card 3: Documentation */}
				<div className="bg-white dark:bg-slate-900 p-5 sm:p-6 rounded-xl border border-slate-200 dark:border-white/10 flex flex-col justify-between hover:border-[#1877F2]/40 transition-colors group">
					<div>
						<div className="w-10 h-10 rounded-xl bg-emerald-50 text-emerald-600 flex items-center justify-center mb-4 group-hover:scale-105 transition-transform">
							<BookOpen className="w-5 h-5" />
						</div>
						<h3 className="text-sm font-bold text-slate-900 dark:text-white mb-1">
							Dokumentasi API
						</h3>
						<p className="text-xs text-slate-500 dark:text-slate-400 leading-relaxed mb-4">
							Akses panduan integrasi cepat dengan snippet cURL, Go, Python,
							Node.js, dan spesifikasi error terstandarisasi.
						</p>
					</div>
					<button
						type="button"
						onClick={() => onNavigate?.("docs")}
						className="inline-flex items-center gap-1.5 text-xs font-semibold text-emerald-600 hover:text-emerald-700 group-hover:translate-x-0.5 transition-all cursor-pointer text-left"
					>
						<span>Lihat Dokumentasi</span>
						<ArrowRight className="w-3.5 h-3.5" />
					</button>
				</div>
			</div>
		</div>
	);
};
