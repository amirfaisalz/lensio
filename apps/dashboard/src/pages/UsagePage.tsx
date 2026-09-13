import {
	BarChart2,
	Building2,
	Clock,
	Layers,
	Plus,
	RefreshCw,
} from "lucide-react";
import type React from "react";
import { useCallback, useEffect, useState } from "react";
import { Modal } from "../components/common/Modal";
import { TableSkeleton } from "../components/common/Skeleton";
import { useAuth } from "../context/AuthContext";
import { api } from "../services/api";
import type { DailyUsage, EndpointUsage } from "../types/api";

export const UsagePage: React.FC = () => {
	const { currentOrg, createOrganization, isInitializing } = useAuth();

	const [daily, setDaily] = useState<DailyUsage[]>([]);
	const [endpoints, setEndpoints] = useState<EndpointUsage[]>([]);
	const [isLoading, setIsLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Org Creation Modal State
	const [isOrgModalOpen, setIsOrgModalOpen] = useState(false);
	const [newOrgName, setNewOrgName] = useState("");
	const [newOrgPlan, setNewOrgPlan] = useState("free");
	const [isCreatingOrg, setIsCreatingOrg] = useState(false);

	const orgId = currentOrg?.id;

	const loadData = useCallback(async () => {
		if (isInitializing) return;
		if (!orgId) {
			setIsLoading(false);
			setDaily([]);
			setEndpoints([]);
			setError(null);
			return;
		}

		try {
			setIsLoading(true);
			setError(null);
			const [dailyRes, endpointRes] = await Promise.all([
				api.fetchDailyUsage(orgId),
				api.fetchEndpointUsage(orgId),
			]);
			setDaily(dailyRes);
			setEndpoints(endpointRes);
		} catch (err) {
			setError(
				err instanceof Error ? err.message : "Failed to load usage analytics",
			);
		} finally {
			setIsLoading(false);
		}
	}, [orgId, isInitializing]);

	useEffect(() => {
		loadData();
	}, [loadData]);

	const handleRefresh = useCallback(() => {
		api.clearCache();
		void loadData();
	}, [loadData]);

	const handleCreateOrg = async (e: React.FormEvent) => {
		e.preventDefault();
		if (!newOrgName.trim() || !createOrganization) return;

		try {
			setIsCreatingOrg(true);
			await createOrganization(newOrgName.trim(), newOrgPlan);
			setNewOrgName("");
			setIsOrgModalOpen(false);
		} catch (err) {
			alert(err instanceof Error ? err.message : "Gagal membuat organisasi");
		} finally {
			setIsCreatingOrg(false);
		}
	};

	if (!isInitializing && !currentOrg) {
		return (
			<div className="space-y-6">
				<div>
					<h2 className="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
						Usage & Analytics
					</h2>
					<p className="text-xs text-slate-500 dark:text-slate-400">
						Historical request volume, endpoint partitioning, and latency
						metrics for current billing period.
					</p>
				</div>

				<div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-white/10 p-8 sm:p-14 text-center max-w-xl mx-auto my-8">
					<div className="w-16 h-16 rounded-2xl bg-[#E7F3FF] dark:bg-[#1877F2]/20 text-[#1877F2] dark:text-[#7aa9f5] flex items-center justify-center mx-auto mb-4">
						<Building2 className="w-8 h-8" />
					</div>
					<h3 className="text-lg font-bold text-slate-900 dark:text-white tracking-tight">
						Organisasi Diperlukan
					</h3>
					<p className="text-xs text-slate-600 dark:text-slate-400 mt-2 leading-relaxed max-w-md mx-auto">
						Anda belum memiliki organisasi. Buat atau pilih organisasi terlebih
						dahulu untuk melihat metrik penggunaan dan analitik kuota API.
					</p>
					<button
						type="button"
						onClick={() => setIsOrgModalOpen(true)}
						className="mt-6 inline-flex items-center gap-2 px-4 py-2.5 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] rounded-xl transition-colors cursor-pointer"
					>
						<Plus className="w-4 h-4" />
						<span>Buat Organisasi Sekarang</span>
					</button>
				</div>

				{/* Create Org Modal */}
				<Modal
					isOpen={isOrgModalOpen}
					onClose={() => setIsOrgModalOpen(false)}
					title="Buat Organisasi Baru"
				>
					<form onSubmit={handleCreateOrg} className="space-y-4">
						<p className="text-xs text-slate-600 dark:text-slate-400">
							Tentukan nama organisasi untuk mengaktifkan kuota API dan analitik
							penggunaan.
						</p>
						<div>
							<label
								htmlFor="new-org-name"
								className="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1"
							>
								Nama Organisasi
							</label>
							<input
								id="new-org-name"
								type="text"
								required
								value={newOrgName}
								onChange={(e) => setNewOrgName(e.target.value)}
								placeholder="contoh: PT Teknologi Maju"
								className="w-full px-3 py-2.5 text-sm border border-slate-300 dark:border-white/15 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1877F2]/25 focus:border-[#1877F2]"
							/>
						</div>
						<div>
							<label
								htmlFor="new-org-plan"
								className="block text-xs font-medium text-slate-700 dark:text-slate-300 mb-1"
							>
								Paket Berlangganan
							</label>
							<select
								id="new-org-plan"
								value={newOrgPlan}
								onChange={(e) => setNewOrgPlan(e.target.value)}
								className="w-full px-3 py-2.5 text-sm border border-slate-300 dark:border-white/15 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#1877F2]/25 focus:border-[#1877F2]"
							>
								<option value="free">Free (100 req/bln)</option>
								<option value="starter">Starter (1.000 req/bln)</option>
								<option value="pro">Pro (10.000 req/bln)</option>
								<option value="enterprise">Enterprise (Unlimited)</option>
							</select>
						</div>
						<div className="flex justify-end gap-2 pt-2">
							<button
								type="button"
								onClick={() => setIsOrgModalOpen(false)}
								className="px-3 py-2 text-xs font-medium text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-white/10 hover:bg-slate-200 dark:hover:bg-white/15 rounded-lg transition-colors cursor-pointer"
							>
								Batal
							</button>
							<button
								type="submit"
								disabled={isCreatingOrg || !newOrgName.trim()}
								className="px-4 py-2 text-xs font-medium text-white bg-[#1877F2] hover:bg-[#166FE5] disabled:opacity-50 rounded-lg transition-colors cursor-pointer"
							>
								{isCreatingOrg ? "Menyimpan..." : "Simpan Organisasi"}
							</button>
						</div>
					</form>
				</Modal>
			</div>
		);
	}

	const maxDailyCount = Math.max(...daily.map((d) => d.total_requests), 10);

	return (
		<div className="space-y-6">
			{/* Top Header */}
			<div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
				<div>
					<h2 className="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
						Usage & Analytics
					</h2>
					<p className="text-xs text-slate-500 dark:text-slate-400">
						Historical request volume, endpoint partitioning, and latency
						metrics for current billing period.
					</p>
				</div>
				<button
					type="button"
					onClick={handleRefresh}
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
				<div className="p-4 bg-rose-50 dark:bg-rose-500/10 border border-rose-200 dark:border-rose-500/20 rounded-xl text-xs text-rose-700 flex items-center justify-between">
					<span>{error}</span>
					<button
						type="button"
						onClick={handleRefresh}
						className="font-semibold underline cursor-pointer"
					>
						Retry
					</button>
				</div>
			)}

			{/* Daily Usage Timeseries Chart */}
			<div className="bg-white dark:bg-slate-900 p-4 sm:p-6 rounded-xl border border-slate-200 dark:border-white/10">
				<div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-6">
					<div>
						<div className="flex items-center gap-2">
							<BarChart2 className="w-4 h-4 text-[#1877F2]" />
							<h3 className="text-sm font-semibold text-slate-900 dark:text-white">
								Daily Request Volume
							</h3>
						</div>
						<p className="text-xs text-slate-500 dark:text-slate-400">
							Distribution of successful vs error requests across the billing
							cycle.
						</p>
					</div>

					<div className="flex flex-wrap items-center gap-3 sm:gap-4 text-xs">
						<div className="flex items-center gap-1.5">
							<span className="w-3 h-3 rounded bg-[#1877F2]" />
							<span className="text-slate-600 dark:text-slate-400">
								Success (&lt; 400)
							</span>
						</div>
						<div className="flex items-center gap-1.5">
							<span className="w-3 h-3 rounded bg-rose-500" />
							<span className="text-slate-600 dark:text-slate-400">
								Error (&ge; 400)
							</span>
						</div>
					</div>
				</div>

				{isLoading ? (
					<div className="h-48 flex items-center justify-center">
						<RefreshCw className="w-6 h-6 animate-spin text-[#1877F2]" />
					</div>
				) : daily.length === 0 ? (
					<div className="h-48 flex flex-col items-center justify-center text-slate-400 dark:text-slate-500 text-xs text-center">
						<p>No activity recorded in this billing cycle yet.</p>
						<p className="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
							Make requests to `/api/v1/ocr/ktp` to view daily activity.
						</p>
					</div>
				) : (
					<div className="overflow-x-auto pb-2">
						<div className="min-w-[420px] space-y-3">
							<div className="h-44 flex items-end gap-2 pt-4 border-b border-slate-100 dark:border-white/10 pb-2">
								{daily.map((d) => {
									const totalH = Math.max(
										8,
										Math.round((d.total_requests / maxDailyCount) * 100),
									);
									const errorPct =
										d.total_requests > 0 ? d.error_count / d.total_requests : 0;
									const successPct = 1 - errorPct;

									return (
										<div
											key={d.date}
											className="flex-1 flex flex-col items-center group relative h-full justify-end"
										>
											{/* Tooltip */}
											<div className="opacity-0 group-hover:opacity-100 transition-opacity absolute -top-12 bg-slate-900 text-white text-[10px] rounded px-2 py-1 pointer-events-none whitespace-nowrap shadow-lg z-10 font-mono">
												<div>{d.date}</div>
												<div>
													Total: {d.total_requests} ({d.success_count} ok,{" "}
													{d.error_count} err)
												</div>
											</div>

											{/* Stacked bar */}
											<div
												className="w-full max-w-[32px] rounded-t overflow-hidden flex flex-col justify-end transition-all duration-300 group-hover:brightness-110"
												style={{ height: `${totalH}%` }}
											>
												{d.error_count > 0 && (
													<div
														className="w-full bg-rose-500"
														style={{ height: `${errorPct * 100}%` }}
													/>
												)}
												<div
													className="w-full bg-[#1877F2]"
													style={{ height: `${successPct * 100}%` }}
												/>
											</div>
										</div>
									);
								})}
							</div>

							{/* Date Labels */}
							<div className="flex justify-between text-[10px] font-mono text-slate-400 dark:text-slate-500">
								<span>{daily[0]?.date}</span>
								<span>{daily[Math.floor(daily.length / 2)]?.date}</span>
								<span>{daily[daily.length - 1]?.date}</span>
							</div>
						</div>
					</div>
				)}
			</div>

			{/* Endpoint Partitioning Table */}
			<div className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-white/10 overflow-hidden">
				<div className="px-4 sm:px-6 py-4 border-b border-slate-100 flex items-center justify-between">
					<div className="flex items-center gap-2">
						<Layers className="w-4 h-4 text-[#1877F2]" />
						<h3 className="text-sm font-semibold text-slate-900 dark:text-white">
							Endpoint Breakdown
						</h3>
					</div>
					<span className="text-xs text-slate-500 dark:text-slate-400">
						{endpoints.length} Active Route(s)
					</span>
				</div>

				<div className="overflow-x-auto">
					<table className="w-full text-left text-xs text-slate-600 dark:text-slate-400 min-w-[560px]">
						<thead className="bg-slate-50 dark:bg-white/5 border-b border-slate-200 dark:border-white/10 text-[11px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
							<tr>
								<th className="px-6 py-3.5">API Endpoint</th>
								<th className="px-6 py-3.5">Request Count</th>
								<th className="px-6 py-3.5">Share of Traffic</th>
								<th className="px-6 py-3.5 text-right">Avg Latency</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-slate-100 dark:divide-white/10 font-medium">
							{isLoading ? (
								<tr>
									<td colSpan={4} className="px-6 py-8">
										<TableSkeleton rows={3} cols={4} />
									</td>
								</tr>
							) : endpoints.length === 0 ? (
								<tr>
									<td
										colSpan={4}
										className="px-6 py-8 text-center text-slate-400 dark:text-slate-500"
									>
										No endpoint requests recorded yet.
									</td>
								</tr>
							) : (
								endpoints.map((ep) => {
									const totalAll =
										endpoints.reduce((acc, e) => acc + e.total_requests, 0) ||
										1;
									const sharePct = (
										(ep.total_requests / totalAll) *
										100
									).toFixed(1);

									return (
										<tr
											key={ep.endpoint}
											className="hover:bg-slate-50/80 dark:hover:bg-white/5 transition-colors"
										>
											<td className="px-6 py-4 font-mono font-semibold text-slate-900 dark:text-white">
												{ep.endpoint}
											</td>
											<td className="px-6 py-4 tabular-nums text-slate-800">
												{ep.total_requests.toLocaleString()}
											</td>
											<td className="px-6 py-4">
												<div className="flex items-center gap-3">
													<div className="w-24 bg-slate-100 dark:bg-white/10 rounded-full h-2 overflow-hidden">
														<div
															className="bg-[#1877F2] h-full rounded-full"
															style={{ width: `${sharePct}%` }}
														/>
													</div>
													<span className="text-slate-500 dark:text-slate-400 font-mono text-[11px]">
														{sharePct}%
													</span>
												</div>
											</td>
											<td className="px-6 py-4 text-right font-mono font-semibold tabular-nums text-slate-900 dark:text-white">
												<span className="inline-flex items-center gap-1">
													<Clock className="w-3.5 h-3.5 text-slate-400 dark:text-slate-500 inline" />
													{ep.avg_latency_ms.toFixed(0)} ms
												</span>
											</td>
										</tr>
									);
								})
							)}
						</tbody>
					</table>
				</div>
			</div>
		</div>
	);
};
