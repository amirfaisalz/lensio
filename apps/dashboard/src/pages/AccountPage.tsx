import {
	Building2,
	Check,
	CreditCard,
	Plus,
	RefreshCw,
	Users,
} from "lucide-react";
import type React from "react";
import { useCallback, useContext, useEffect, useState } from "react";
import { Badge } from "../components/common/Badge";
import { Modal } from "../components/common/Modal";
import { Skeleton } from "../components/common/Skeleton";
import { AuthContext } from "../context/AuthContext";
import { api } from "../services/api";
import type {
	OrganizationDetails,
	PlanDetails,
	UserMember,
} from "../types/api";

export const AccountPage: React.FC = () => {
	const auth = useContext(AuthContext);
	const currentOrg = auth?.currentOrg;
	const createOrganization = auth?.createOrganization;

	const [org, setOrg] = useState<OrganizationDetails | null>(null);
	const [plan, setPlan] = useState<PlanDetails | null>(null);
	const [members, setMembers] = useState<UserMember[]>([]);
	const [isLoading, setIsLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Org Creation Modal State (from Placeholder)
	const [isOrgModalOpen, setIsOrgModalOpen] = useState(false);
	const [newOrgName, setNewOrgName] = useState("");
	const [newOrgPlan, setNewOrgPlan] = useState("free");
	const [isCreatingOrg, setIsCreatingOrg] = useState(false);

	// Plan update state
	const [selectedPlan, setSelectedPlan] = useState<string>("free");
	const [isUpdatingPlan, setIsUpdatingPlan] = useState(false);
	const [planUpdateSuccess, setPlanUpdateSuccess] = useState(false);

	const loadData = useCallback(async () => {
		if (auth !== undefined && !currentOrg) {
			setIsLoading(false);
			setOrg(null);
			setPlan(null);
			setMembers([]);
			setError(null);
			return;
		}

		try {
			setIsLoading(true);
			setError(null);
			const [orgRes, planRes, membersRes] = await Promise.all([
				api.fetchAccount(currentOrg?.id),
				api.fetchAccountPlan(currentOrg?.id),
				api.fetchAccountMembers(currentOrg?.id),
			]);
			setOrg(orgRes);
			setPlan(planRes);
			setSelectedPlan(planRes.plan_code);
			setMembers(membersRes);
		} catch (err) {
			const errMsg =
				err instanceof Error
					? err.message
					: "Failed to load account information";
			if (errMsg.toLowerCase().includes("not found")) {
				setOrg(null);
				setPlan(null);
				setMembers([]);
				setError(null);
			} else {
				setError(errMsg);
			}
		} finally {
			setIsLoading(false);
		}
	}, [auth, currentOrg]);

	useEffect(() => {
		loadData();
	}, [loadData]);

	const handleCreateOrg = async (e: React.FormEvent) => {
		e.preventDefault();
		const name = newOrgName.trim();
		if (!name) return;

		try {
			setIsCreatingOrg(true);
			if (createOrganization) {
				await createOrganization(name, newOrgPlan);
			} else {
				await api.createOrganization({ name, plan_code: newOrgPlan });
			}
			setNewOrgName("");
			setIsOrgModalOpen(false);
			await loadData();
		} catch (err) {
			alert(err instanceof Error ? err.message : "Gagal membuat organisasi");
		} finally {
			setIsCreatingOrg(false);
		}
	};

	const handleUpdatePlan = async () => {
		if (!plan || selectedPlan === plan.plan_code) return;

		try {
			setIsUpdatingPlan(true);
			setPlanUpdateSuccess(false);
			await api.updateAccountPlan(selectedPlan, currentOrg?.id);
			setPlanUpdateSuccess(true);
			await loadData();
			setTimeout(() => setPlanUpdateSuccess(false), 3000);
		} catch (err) {
			alert(
				err instanceof Error
					? err.message
					: "Failed to update subscription plan",
			);
		} finally {
			setIsUpdatingPlan(false);
		}
	};

	const planOptions = [
		{
			code: "free",
			name: "Free Tier",
			quota: "100 requests/mo",
			rateLimit: "10 req/min",
			price: "$0 / mo",
			desc: "Ideal for testing and development integration.",
		},
		{
			code: "starter",
			name: "Starter Tier",
			quota: "1,000 requests/mo",
			rateLimit: "30 req/min",
			price: "$29 / mo",
			desc: "For early-stage products and growing MVPs.",
		},
		{
			code: "pro",
			name: "Pro Tier",
			quota: "10,000 requests/mo",
			rateLimit: "100 req/min",
			price: "$199 / mo",
			desc: "Production scale with high concurrency limits.",
		},
	];

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
				<div>
					<h2 className="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
						Account & Settings
					</h2>
					<p className="text-xs text-slate-500 dark:text-slate-400">
						Tenant organization profile, subscription tier, and registered team
						members.
					</p>
				</div>
				<button
					type="button"
					onClick={loadData}
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
						onClick={loadData}
						className="font-semibold underline"
					>
						Retry
					</button>
				</div>
			)}

			{/* Placeholder Empty State if No Organization */}
			{!isLoading && !org ? (
				<div className="space-y-6">
					<div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-white/10 p-8 sm:p-14 text-center max-w-xl mx-auto my-4">
						<div className="w-16 h-16 rounded-2xl bg-[#E7F3FF] dark:bg-[#1877F2]/20 text-[#1877F2] dark:text-[#7aa9f5] flex items-center justify-center mx-auto mb-4">
							<Building2 className="w-8 h-8" />
						</div>
						<h3 className="text-lg font-bold text-slate-900 dark:text-white tracking-tight">
							Belum Ada Organisasi
						</h3>
						<p className="text-xs text-slate-600 dark:text-slate-400 mt-2 leading-relaxed max-w-md mx-auto">
							Akun Anda belum terdaftar dalam organisasi mana pun. Buat
							organisasi baru untuk mulai mengelola profil tenant, memilih paket
							kuota API, dan mengundang anggota tim.
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

					<div className="grid grid-cols-1 md:grid-cols-3 gap-4">
						<div className="bg-white dark:bg-slate-900 p-5 rounded-xl border border-slate-200 dark:border-white/10">
							<div className="w-8 h-8 rounded-lg bg-blue-50 text-[#1877F2] flex items-center justify-center mb-3">
								<Building2 className="w-4 h-4" />
							</div>
							<h4 className="text-xs font-bold text-slate-900 dark:text-white mb-1">
								Profil Tenant
							</h4>
							<p className="text-[11px] text-slate-500 dark:text-slate-400 leading-relaxed">
								Slug dan ID unik untuk isolasi data, pelacakan API key, dan
								audit trail pemrosesan OCR dokumen identitas.
							</p>
						</div>

						<div className="bg-white dark:bg-slate-900 p-5 rounded-xl border border-slate-200 dark:border-white/10">
							<div className="w-8 h-8 rounded-lg bg-indigo-50 text-indigo-600 flex items-center justify-center mb-3">
								<CreditCard className="w-4 h-4" />
							</div>
							<h4 className="text-xs font-bold text-slate-900 dark:text-white mb-1">
								Paket & Kuota Bulanan
							</h4>
							<p className="text-[11px] text-slate-500 dark:text-slate-400 leading-relaxed">
								Pilihan tier fleksibel mulai dari Free (100 req/bln), Starter
								(1.000 req/bln), hingga Pro (10.000 req/bln).
							</p>
						</div>

						<div className="bg-white dark:bg-slate-900 p-5 rounded-xl border border-slate-200 dark:border-white/10">
							<div className="w-8 h-8 rounded-lg bg-emerald-50 text-emerald-600 flex items-center justify-center mb-3">
								<Users className="w-4 h-4" />
							</div>
							<h4 className="text-xs font-bold text-slate-900 dark:text-white mb-1">
								Manajemen Anggota
							</h4>
							<p className="text-[11px] text-slate-500 dark:text-slate-400 leading-relaxed">
								Kelola akses tim developer Anda dengan otorisasi berbasis
								SpiceDB ReBAC terintegrasi.
							</p>
						</div>
					</div>

					{/* Create Org Modal */}
					<Modal
						isOpen={isOrgModalOpen}
						onClose={() => setIsOrgModalOpen(false)}
						title="Buat Organisasi Baru"
					>
						<form onSubmit={handleCreateOrg} className="space-y-4">
							<p className="text-xs text-slate-600 dark:text-slate-400">
								Tentukan nama organisasi untuk mengaktifkan kuota API dan
								mengelola akun Anda.
							</p>

							<div>
								<label
									htmlFor="account-new-org-name"
									className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1"
								>
									Nama Organisasi / Perusahaan
								</label>
								<div className="relative">
									<Building2 className="w-4 h-4 text-slate-400 dark:text-slate-500 absolute left-3 top-2.5" />
									<input
										id="account-new-org-name"
										type="text"
										placeholder="misal: PT Fintech Nusantara"
										value={newOrgName}
										onChange={(e) => setNewOrgName(e.target.value)}
										className="w-full pl-9 pr-3 py-2.5 text-sm border border-slate-300 dark:border-white/15 rounded-lg focus:outline-none focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/25"
									/>
								</div>
							</div>

							<div>
								<label
									htmlFor="account-org-plan"
									className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1"
								>
									Paket Langganan Awal
								</label>
								<select
									id="account-org-plan"
									value={newOrgPlan}
									onChange={(e) => setNewOrgPlan(e.target.value)}
									className="w-full px-3 py-2.5 text-sm border border-slate-300 dark:border-white/15 rounded-lg bg-white dark:bg-slate-800 focus:outline-none focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/25"
								>
									<option value="free">
										Free Tier (100 req/bulan, 10 req/min) - Gratis
									</option>
									<option value="starter">
										Starter Tier (1,000 req/bulan, 30 req/min)
									</option>
									<option value="pro">
										Pro Tier (10,000 req/bulan, 100 req/min)
									</option>
								</select>
							</div>

							<div className="flex justify-end gap-2 pt-2">
								<button
									type="button"
									onClick={() => setIsOrgModalOpen(false)}
									className="px-3 py-2 text-xs font-medium text-slate-600 dark:text-slate-300 hover:text-slate-800 dark:hover:text-white bg-white dark:bg-white/5 border border-slate-200 dark:border-white/10 rounded-lg hover:bg-slate-50 dark:hover:bg-white/10 transition-colors"
								>
									Batal
								</button>
								<button
									type="submit"
									disabled={!newOrgName.trim() || isCreatingOrg}
									className="px-4 py-2 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] disabled:opacity-50 rounded-lg transition-colors cursor-pointer"
								>
									{isCreatingOrg ? "Membuat..." : "Buat Organisasi & Simpan"}
								</button>
							</div>
						</form>
					</Modal>
				</div>
			) : (
				<>
					{/* Organization Profile Card */}
					<div className="bg-white dark:bg-slate-900 p-4 sm:p-6 rounded-xl border border-slate-200 dark:border-white/10">
						<div className="flex items-center gap-2 mb-4">
							<Building2 className="w-4 h-4 text-[#1877F2]" />
							<h3 className="text-sm font-semibold text-slate-900 dark:text-white">
								Organization Profile
							</h3>
						</div>

						{isLoading ? (
							<div className="space-y-2">
								<Skeleton className="h-4 w-48" />
								<Skeleton className="h-4 w-64" />
							</div>
						) : org ? (
							<div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4 text-xs">
								<div className="p-3 bg-slate-50 dark:bg-white/5 rounded-lg border border-slate-100 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Organization Name
									</span>
									<span className="font-bold text-slate-900 dark:text-white text-sm">
										{org.organization_name}
									</span>
								</div>
								<div className="p-3 bg-slate-50 dark:bg-white/5 rounded-lg border border-slate-100 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Tenant Slug
									</span>
									<span className="font-mono text-slate-700 dark:text-slate-300">
										{org.slug}
									</span>
								</div>
								<div className="p-3 bg-slate-50 dark:bg-white/5 rounded-lg border border-slate-100 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Organization ID
									</span>
									<span className="font-mono text-[11px] text-slate-700 dark:text-slate-300 break-all">
										{org.organization_id}
									</span>
								</div>
								<div className="p-3 bg-slate-50 dark:bg-white/5 rounded-lg border border-slate-100 dark:border-white/10">
									<span className="text-slate-400 dark:text-slate-500 block text-[10px] uppercase font-semibold">
										Active API Keys
									</span>
									<span className="font-bold text-[#1877F2] text-sm tabular-nums">
										{org.active_keys_count} active
									</span>
								</div>
							</div>
						) : null}
					</div>

					{/* Subscription Plan Switcher */}
					<div className="bg-white dark:bg-slate-900 p-4 sm:p-6 rounded-xl border border-slate-200 dark:border-white/10">
						<div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-4">
							<div>
								<div className="flex items-center gap-2">
									<CreditCard className="w-4 h-4 text-[#1877F2]" />
									<h3 className="text-sm font-semibold text-slate-900 dark:text-white">
										Subscription Plan & Quotas
									</h3>
								</div>
								<p className="text-xs text-slate-500 dark:text-slate-400">
									Select an API tier to instantly adjust your monthly quota and
									per-minute throughput limits.
								</p>
							</div>

							{planUpdateSuccess && (
								<span className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-600 dark:text-emerald-300 bg-emerald-50 dark:bg-emerald-500/10 px-2.5 py-1 rounded-md border border-emerald-200 dark:border-emerald-500/20 self-start sm:self-auto">
									<Check className="w-3.5 h-3.5" /> Plan Updated Successfully
								</span>
							)}
						</div>

						<div className="grid grid-cols-1 md:grid-cols-3 gap-3 sm:gap-4 mb-6">
							{planOptions.map((opt) => {
								const isCurrent = plan?.plan_code === opt.code;
								const isSelected = selectedPlan === opt.code;

								return (
									<button
										type="button"
										key={opt.code}
										onClick={() => setSelectedPlan(opt.code)}
										className={`text-left p-4 rounded-xl border cursor-pointer transition-all ${
											isSelected
												? "border-[#1877F2] ring-2 ring-[#1877F2]/20 bg-[#E7F3FF]/20 dark:bg-[#1877F2]/10"
												: "border-slate-200 dark:border-white/10 hover:border-slate-300 dark:hover:border-white/25 bg-white dark:bg-slate-900"
										}`}
									>
										<div className="flex items-center justify-between mb-2">
											<span className="font-bold text-slate-900 dark:text-white text-sm">
												{opt.name}
											</span>
											{isCurrent && (
												<Badge variant="success">Current Plan</Badge>
											)}
										</div>

										<div className="text-lg font-bold text-slate-900 dark:text-white mb-1">
											{opt.price}
										</div>
										<p className="text-xs text-slate-500 dark:text-slate-400 mb-3">
											{opt.desc}
										</p>

										<div className="space-y-1 text-xs text-slate-700 dark:text-slate-300 border-t border-slate-100 dark:border-white/10 pt-3 tabular-nums">
											<div className="flex justify-between">
												<span className="text-slate-500 dark:text-slate-400">
													Monthly Quota:
												</span>
												<strong className="text-slate-900 dark:text-white">
													{opt.quota}
												</strong>
											</div>
											<div className="flex justify-between">
												<span className="text-slate-500 dark:text-slate-400">
													Rate Limit:
												</span>
												<strong className="text-slate-900 dark:text-white">
													{opt.rateLimit}
												</strong>
											</div>
										</div>
									</button>
								);
							})}
						</div>

						{plan && selectedPlan !== plan.plan_code && (
							<div className="flex flex-col-reverse sm:flex-row items-stretch sm:items-center justify-end gap-2 sm:gap-3 pt-4 border-t border-slate-100">
								<button
									type="button"
									onClick={() => setSelectedPlan(plan.plan_code)}
									className="px-4 py-2 text-xs font-medium text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:text-white text-center cursor-pointer"
								>
									Reset
								</button>
								<button
									type="button"
									onClick={handleUpdatePlan}
									disabled={isUpdatingPlan}
									className="px-4 py-2 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] rounded-lg transition-colors text-center cursor-pointer"
								>
									{isUpdatingPlan
										? "Updating Plan..."
										: `Switch to ${selectedPlan.toUpperCase()}`}
								</button>
							</div>
						)}
					</div>

					{/* Team Members List */}
					<div className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-white/10 overflow-hidden">
						<div className="px-4 sm:px-6 py-4 border-b border-slate-100 flex items-center justify-between">
							<div className="flex items-center gap-2">
								<Users className="w-4 h-4 text-[#1877F2]" />
								<h3 className="text-sm font-semibold text-slate-900 dark:text-white">
									Organization Members
								</h3>
							</div>
							<span className="text-xs text-slate-500 dark:text-slate-400">
								{members.length} Member(s)
							</span>
						</div>

						<div className="overflow-x-auto">
							<table className="w-full text-left text-xs text-slate-600 dark:text-slate-400 min-w-[540px]">
								<thead className="bg-slate-50 dark:bg-white/5 border-b border-slate-200 dark:border-white/10 text-[11px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
									<tr>
										<th className="px-6 py-3.5">Name</th>
										<th className="px-6 py-3.5">Email</th>
										<th className="px-6 py-3.5">Role</th>
										<th className="px-6 py-3.5">Joined Date</th>
									</tr>
								</thead>
								<tbody className="divide-y divide-slate-100 dark:divide-white/10 font-medium">
									{isLoading ? (
										<tr>
											<td colSpan={4} className="px-6 py-8">
												<Skeleton className="h-6 w-full my-2" />
											</td>
										</tr>
									) : members.length === 0 ? (
										<tr>
											<td
												colSpan={4}
												className="px-6 py-8 text-center text-slate-400 dark:text-slate-500"
											>
												No members listed.
											</td>
										</tr>
									) : (
										members.map((m) => (
											<tr
												key={m.id}
												className="hover:bg-slate-50/80 dark:hover:bg-white/5 transition-colors"
											>
												<td className="px-6 py-3.5 font-semibold text-slate-900 dark:text-white">
													{m.full_name}
												</td>
												<td className="px-6 py-3.5 font-mono text-slate-600 dark:text-slate-400">
													{m.email}
												</td>
												<td className="px-6 py-3.5">
													<Badge
														variant={m.role === "owner" ? "info" : "neutral"}
													>
														{m.role}
													</Badge>
												</td>
												<td className="px-6 py-3.5 text-slate-500 dark:text-slate-400 tabular-nums">
													{new Date(m.created_at).toLocaleDateString(
														undefined,
														{
															month: "short",
															day: "numeric",
															year: "numeric",
														},
													)}
												</td>
											</tr>
										))
									)}
								</tbody>
							</table>
						</div>
					</div>
				</>
			)}
		</div>
	);
};
