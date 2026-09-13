import {
	AlertTriangle,
	Building2,
	CheckCircle2,
	ChevronLeft,
	ChevronRight,
	Eye,
	Plus,
	RefreshCw,
	ShieldCheck,
	XCircle,
} from "lucide-react";
import type React from "react";
import { useCallback, useEffect, useState } from "react";
import { Modal } from "../components/common/Modal";
import { TableSkeleton } from "../components/common/Skeleton";
import { useAuth } from "../context/AuthContext";
import { api } from "../services/api";
import type { UsageRecord } from "../types/api";

export const RequestsPage: React.FC = () => {
	const { currentOrg, createOrganization, isInitializing } = useAuth();

	const [records, setRecords] = useState<UsageRecord[]>([]);
	const [total, setTotal] = useState(0);
	const [limit] = useState(25);
	const [offset, setOffset] = useState(0);
	const [statusCodeFilter, setStatusCodeFilter] = useState<number | undefined>(
		undefined,
	);
	const [endpointFilter, setEndpointFilter] = useState("");
	const [isLoading, setIsLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Detail Modal State
	const [selectedRecord, setSelectedRecord] = useState<UsageRecord | null>(
		null,
	);

	// Org Creation Modal State
	const [isOrgModalOpen, setIsOrgModalOpen] = useState(false);
	const [newOrgName, setNewOrgName] = useState("");
	const [newOrgPlan, setNewOrgPlan] = useState("free");
	const [isCreatingOrg, setIsCreatingOrg] = useState(false);

	const orgId = currentOrg?.id;

	const loadRecords = useCallback(async () => {
		if (isInitializing) return;
		if (!orgId) {
			setIsLoading(false);
			setRecords([]);
			setTotal(0);
			setError(null);
			return;
		}

		try {
			setIsLoading(true);
			setError(null);
			const res = await api.fetchUsageRecords({
				limit,
				offset,
				status_code: statusCodeFilter,
				endpoint: endpointFilter || undefined,
				org_id: orgId,
			});
			setRecords(res.data || []);
			setTotal(res.total || 0);
		} catch (err) {
			setError(
				err instanceof Error ? err.message : "Failed to load request logs",
			);
		} finally {
			setIsLoading(false);
		}
	}, [orgId, isInitializing, limit, offset, statusCodeFilter, endpointFilter]);

	useEffect(() => {
		loadRecords();
	}, [loadRecords]);

	const handleRefresh = useCallback(() => {
		api.clearCache();
		void loadRecords();
	}, [loadRecords]);

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
						Live Request Logs
					</h2>
					<p className="text-xs text-slate-500 dark:text-slate-400">
						Audit trail and real-time inspection for every OCR processing
						request.
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
						dahulu untuk melihat riwayat log permintaan OCR.
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
							Tentukan nama organisasi untuk mengaktifkan kuota API dan log
							permintaan.
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
								className="px-3 py-2 text-xs font-medium text-slate-700 dark:text-slate-300 bg-slate-100 hover:bg-slate-200 rounded-lg transition-colors cursor-pointer"
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

	const getStatusBadge = (code: number) => {
		if (code >= 200 && code < 300) {
			return (
				<span className="inline-flex items-center gap-1 text-emerald-700 dark:text-emerald-300 bg-emerald-50 dark:bg-emerald-500/10 border border-emerald-200 dark:border-emerald-500/20 px-2 py-0.5 rounded text-xs font-semibold tabular-nums">
					<CheckCircle2 className="w-3 h-3 text-emerald-600" />
					{code} OK
				</span>
			);
		}
		if (code >= 400 && code < 500) {
			return (
				<span className="inline-flex items-center gap-1 text-amber-700 dark:text-amber-300 bg-amber-50 dark:bg-amber-500/10 border border-amber-200 dark:border-amber-500/20 px-2 py-0.5 rounded text-xs font-semibold tabular-nums">
					<AlertTriangle className="w-3 h-3 text-amber-600" />
					{code}
				</span>
			);
		}
		return (
			<span className="inline-flex items-center gap-1 text-rose-700 dark:text-rose-300 bg-rose-50 dark:bg-rose-500/10 border border-rose-200 dark:border-rose-500/20 px-2 py-0.5 rounded text-xs font-semibold tabular-nums">
				<XCircle className="w-3 h-3 text-rose-600" />
				{code}
			</span>
		);
	};

	const totalPages = Math.ceil(total / limit) || 1;
	const currentPage = Math.floor(offset / limit) + 1;

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 sm:gap-4">
				<div>
					<h2 className="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
						Requests Explorer
					</h2>
					<p className="text-xs text-slate-500 dark:text-slate-400">
						Audit trail of API requests with status codes, latency, and non-PII
						execution metadata.
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
				<div className="p-4 bg-rose-50 dark:bg-rose-500/10 border border-rose-200 dark:border-rose-500/20 rounded-xl text-xs text-rose-700 dark:text-rose-300 flex items-center justify-between">
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

			{/* Filter Bar */}
			<div className="bg-white dark:bg-slate-900 p-4 rounded-xl border border-slate-200 dark:border-white/10 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
				<div className="flex flex-col xs:flex-row items-stretch xs:items-center gap-2.5 w-full sm:w-auto">
					{/* Status Code Filter */}
					<div className="flex items-center gap-2">
						<span className="text-slate-500 dark:text-slate-400 font-medium shrink-0">
							Status:
						</span>
						<select
							value={statusCodeFilter ?? ""}
							onChange={(e) => {
								const val = e.target.value ? Number(e.target.value) : undefined;
								setStatusCodeFilter(val);
								setOffset(0);
							}}
							className="px-2.5 py-1.5 border border-slate-300 dark:border-white/15 rounded-lg text-xs bg-white dark:bg-slate-800 text-slate-800 dark:text-slate-100 focus:outline-none focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/25 w-full xs:w-auto cursor-pointer"
						>
							<option value="">All Statuses</option>
							<option value="200">200 OK</option>
							<option value="400">400 Bad Request</option>
							<option value="401">401 Unauthorized</option>
							<option value="403">403 Forbidden</option>
							<option value="422">422 Unsupported</option>
							<option value="429">429 Rate Limited</option>
							<option value="500">500 Server Error</option>
							<option value="502">502 Provider Error</option>
						</select>
					</div>

					{/* Endpoint text filter */}
					<div className="flex items-center gap-2">
						<span className="text-slate-500 dark:text-slate-400 font-medium shrink-0">
							Route:
						</span>
						<input
							type="text"
							placeholder="e.g. /api/v1/ocr/ktp, /api/v1/ocr/sim"
							value={endpointFilter}
							onChange={(e) => {
								setEndpointFilter(e.target.value.trim());
								setOffset(0);
							}}
							className="px-2.5 py-1.5 border border-slate-300 dark:border-white/15 rounded-lg text-xs font-mono bg-white dark:bg-slate-800 text-slate-800 dark:text-slate-100 focus:outline-none focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/25 w-full xs:w-auto"
						/>
					</div>
				</div>

				<div className="text-xs text-slate-500 dark:text-slate-400 tabular-nums">
					Showing <strong className="text-slate-800">{records.length}</strong>{" "}
					of {total.toLocaleString()} records
				</div>
			</div>

			{/* Requests Table */}
			<div className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-white/10 overflow-hidden">
				<div className="overflow-x-auto">
					<table className="w-full text-left text-xs text-slate-600 dark:text-slate-400 min-w-[660px]">
						<thead className="bg-slate-50 dark:bg-white/5 border-b border-slate-200 dark:border-white/10 text-[11px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
							<tr>
								<th className="px-6 py-3.5">Status</th>
								<th className="px-6 py-3.5">Endpoint</th>
								<th className="px-6 py-3.5">Request ID</th>
								<th className="px-6 py-3.5">Latency</th>
								<th className="px-6 py-3.5">Timestamp</th>
								<th className="px-6 py-3.5 text-right">Details</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-slate-100 dark:divide-white/10 font-medium">
							{isLoading ? (
								<tr>
									<td colSpan={6} className="px-6 py-8">
										<TableSkeleton rows={5} cols={6} />
									</td>
								</tr>
							) : records.length === 0 ? (
								<tr>
									<td
										colSpan={6}
										className="px-6 py-12 text-center text-slate-400 dark:text-slate-500"
									>
										No matching requests found.
									</td>
								</tr>
							) : (
								records.map((r) => (
									<tr
										key={r.id}
										onClick={() => setSelectedRecord(r)}
										className="hover:bg-slate-50/80 dark:hover:bg-white/5 cursor-pointer transition-colors"
									>
										<td className="px-6 py-3.5">
											{getStatusBadge(r.status_code)}
										</td>
										<td className="px-6 py-3.5 font-mono font-semibold text-slate-900 dark:text-white">
											{r.endpoint}
										</td>
										<td className="px-6 py-3.5 font-mono text-[11px] text-slate-500 dark:text-slate-400">
											{r.request_id || "n/a"}
										</td>
										<td className="px-6 py-3.5 font-mono text-slate-700 dark:text-slate-300 tabular-nums">
											{r.latency_ms} ms
										</td>
										<td className="px-6 py-3.5 text-slate-500 dark:text-slate-400 tabular-nums">
											{new Date(r.timestamp).toLocaleDateString(undefined, {
												month: "short",
												day: "numeric",
												hour: "2-digit",
												minute: "2-digit",
												second: "2-digit",
											})}
										</td>
										<td className="px-6 py-3.5 text-right">
											<button
												type="button"
												onClick={(e) => {
													e.stopPropagation();
													setSelectedRecord(r);
												}}
												className="text-slate-400 dark:text-slate-500 hover:text-[#1877F2] p-1 rounded hover:bg-slate-100 dark:hover:bg-white/10 transition-colors"
												title="View request details"
											>
												<Eye className="w-4 h-4" />
											</button>
										</td>
									</tr>
								))
							)}
						</tbody>
					</table>
				</div>

				{/* Pagination Footer */}
				<div className="px-4 sm:px-6 py-3.5 border-t border-slate-100 dark:border-white/10 bg-slate-50/50 dark:bg-white/[0.02] flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
					<div>
						Page <strong className="text-slate-800">{currentPage}</strong> of{" "}
						{totalPages}
					</div>

					<div className="flex items-center gap-2">
						<button
							type="button"
							disabled={offset === 0 || isLoading}
							onClick={() => setOffset(Math.max(0, offset - limit))}
							className="p-2 sm:p-1.5 border border-slate-200 dark:border-white/10 bg-white dark:bg-white/5 rounded-md disabled:opacity-40 hover:bg-slate-50 dark:hover:bg-white/10 transition-colors cursor-pointer"
						>
							<ChevronLeft className="w-4 h-4" />
						</button>
						<button
							type="button"
							disabled={offset + limit >= total || isLoading}
							onClick={() => setOffset(offset + limit)}
							className="p-2 sm:p-1.5 border border-slate-200 dark:border-white/10 bg-white dark:bg-white/5 rounded-md disabled:opacity-40 hover:bg-slate-50 dark:hover:bg-white/10 transition-colors cursor-pointer"
						>
							<ChevronRight className="w-4 h-4" />
						</button>
					</div>
				</div>
			</div>

			{/* Request Details Modal */}
			{selectedRecord && (
				<Modal
					isOpen={true}
					onClose={() => setSelectedRecord(null)}
					title="Request Audit Details"
					footer={
						<button
							type="button"
							onClick={() => setSelectedRecord(null)}
							className="w-full sm:w-auto px-4 py-2 text-xs font-semibold text-slate-700 dark:text-slate-300 bg-white border border-slate-200 rounded-lg hover:bg-slate-50 cursor-pointer text-center"
						>
							Close
						</button>
					}
				>
					<div className="space-y-4">
						<div className="flex items-center justify-between p-3 bg-slate-50 dark:bg-white/5 rounded-lg border border-slate-200 dark:border-white/10">
							<span className="text-xs text-slate-500 dark:text-slate-400">
								HTTP Status
							</span>
							{getStatusBadge(selectedRecord.status_code)}
						</div>

						<div className="space-y-2 text-xs">
							<div className="flex items-center justify-between py-1 border-b border-slate-100 dark:border-white/10 gap-2">
								<span className="text-slate-500 dark:text-slate-400 shrink-0">
									Request ID
								</span>
								<span className="font-mono text-slate-900 dark:text-white font-semibold break-all text-right">
									{selectedRecord.request_id}
								</span>
							</div>
							<div className="flex items-center justify-between py-1 border-b border-slate-100 dark:border-white/10 gap-2">
								<span className="text-slate-500 dark:text-slate-400 shrink-0">
									Target Endpoint
								</span>
								<span className="font-mono text-slate-900 dark:text-white break-all text-right">
									{selectedRecord.endpoint}
								</span>
							</div>
							<div className="flex items-center justify-between py-1 border-b border-slate-100 dark:border-white/10 gap-2">
								<span className="text-slate-500 dark:text-slate-400 shrink-0">
									Execution Latency
								</span>
								<span className="font-mono text-slate-900 dark:text-white">
									{selectedRecord.latency_ms} ms
								</span>
							</div>
							<div className="flex items-center justify-between py-1 border-b border-slate-100 dark:border-white/10 gap-2">
								<span className="text-slate-500 dark:text-slate-400 shrink-0">
									Timestamp
								</span>
								<span className="font-mono text-slate-900 dark:text-white text-right">
									{new Date(selectedRecord.timestamp).toISOString()}
								</span>
							</div>
							<div className="flex items-center justify-between py-1 border-b border-slate-100 dark:border-white/10 gap-2">
								<span className="text-slate-500 dark:text-slate-400 shrink-0">
									API Key UUID
								</span>
								<span className="font-mono text-slate-700 dark:text-slate-300 break-all text-right">
									{selectedRecord.api_key_id || "System / Direct"}
								</span>
							</div>
						</div>

						{/* Privacy note */}
						<div className="p-3 bg-emerald-50 dark:bg-emerald-500/10 border border-emerald-200 dark:border-emerald-500/20 rounded-lg flex items-start gap-2 text-xs text-emerald-800 dark:text-emerald-300">
							<ShieldCheck className="w-4 h-4 text-emerald-600 shrink-0 mt-0.5" />
							<div>
								<strong>Strict Privacy & Data Minimization:</strong> Raw images
								and identity data (NIK, full names, addresses) are processed
								strictly in ephemeral memory and never persisted into queryable
								databases or logs.
							</div>
						</div>
					</div>
				</Modal>
			)}
		</div>
	);
};
