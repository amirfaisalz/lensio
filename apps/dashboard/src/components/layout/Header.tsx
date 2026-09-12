import { Building2, ChevronDown, LogOut, Menu, Plus } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../context/AuthContext";
import { Modal } from "../common/Modal";

interface HeaderProps {
	title: string;
	subtitle?: string;
	onOpenMobileNav?: () => void;
}

export const Header: React.FC<HeaderProps> = ({
	title,
	subtitle,
	onOpenMobileNav,
}) => {
	const navigate = useNavigate();
	const { currentOrg, oidcUser, apiKey, createOrganization, logout } =
		useAuth();

	const [isOrgModalOpen, setIsOrgModalOpen] = useState(false);
	const [newOrgName, setNewOrgName] = useState("");
	const [newOrgPlan, setNewOrgPlan] = useState("free");
	const [isCreatingOrg, setIsCreatingOrg] = useState(false);

	const handleCreateOrg = (e: React.FormEvent) => {
		e.preventDefault();
		if (!newOrgName.trim()) return;

		setIsCreatingOrg(true);
		createOrganization(newOrgName.trim(), newOrgPlan);
		setNewOrgName("");
		setIsCreatingOrg(false);
		setIsOrgModalOpen(false);
	};

	const handleLogout = () => {
		logout();
		navigate("/login");
	};

	// Determine display name / email
	const userIdentifier =
		oidcUser?.email || (apiKey ? "API Key Connected" : "Pengguna");
	const userInitial = userIdentifier.charAt(0).toUpperCase();

	return (
		<header className="h-16 bg-white border-b border-slate-200 px-4 sm:px-6 lg:px-8 flex items-center justify-between sticky top-0 z-30">
			{/* Left: Hamburger (mobile/tablet) + Page Title */}
			<div className="flex items-center gap-2.5 sm:gap-3 min-w-0 mr-2">
				<button
					type="button"
					onClick={onOpenMobileNav}
					aria-label="Buka menu navigasi"
					className="p-2 -ml-1 text-slate-600 hover:text-slate-900 hover:bg-slate-100 rounded-lg lg:hidden cursor-pointer shrink-0 transition-colors"
				>
					<Menu className="w-5 h-5" />
				</button>
				<div className="min-w-0">
					<h1 className="text-base sm:text-lg font-bold text-slate-900 tracking-tight truncate">
						{title}
					</h1>
					{subtitle && (
						<p className="text-[11px] text-slate-500 hidden sm:block truncate">
							{subtitle}
						</p>
					)}
				</div>
			</div>

			{/* Right: Clean & Minimalist Action Elements */}
			<div className="flex items-center gap-2 sm:gap-3 shrink-0">
				{/* Organization Selector / Creation Button */}
				{currentOrg ? (
					<button
						type="button"
						onClick={() => setIsOrgModalOpen(true)}
						className="inline-flex items-center gap-1.5 sm:gap-2 px-2.5 sm:px-3 py-1.5 text-xs font-medium rounded-lg border border-slate-200 bg-slate-50 hover:bg-slate-100 text-slate-700 transition-colors cursor-pointer"
					>
						<Building2 className="w-3.5 h-3.5 text-slate-500 shrink-0" />
						<span className="font-semibold text-slate-900 max-w-[80px] xs:max-w-[110px] sm:max-w-[140px] truncate">
							{currentOrg.name}
						</span>
						<span className="text-[10px] px-1.5 py-0.2 rounded bg-[#E7F3FF] text-[#1877F2] font-semibold uppercase hidden xs:inline-block">
							{currentOrg.planCode}
						</span>
						<ChevronDown className="w-3 h-3 text-slate-400 ml-0.5 shrink-0" />
					</button>
				) : (
					<button
						type="button"
						onClick={() => setIsOrgModalOpen(true)}
						className="inline-flex items-center gap-1.5 px-2.5 sm:px-3 py-1.5 text-xs font-semibold rounded-lg bg-[#E7F3FF] text-[#1877F2] hover:bg-[#d4e9ff] transition-colors cursor-pointer"
					>
						<Plus className="w-3.5 h-3.5" />
						<span className="hidden xs:inline">Buat Organisasi</span>
						<span className="xs:hidden">Organisasi</span>
					</button>
				)}

				{/* User Profile & Logout */}
				<div className="flex items-center gap-1.5 sm:gap-2 pl-1.5 sm:pl-2 border-l border-slate-200">
					<div className="flex items-center gap-2 px-2 sm:px-2.5 py-1 rounded-lg bg-slate-50 border border-slate-200 text-slate-700">
						<div className="w-6 h-6 rounded-full bg-[#1877F2] text-white flex items-center justify-center text-[11px] font-bold shrink-0">
							{userInitial}
						</div>
						<span className="text-xs font-medium max-w-[150px] truncate hidden md:inline">
							{userIdentifier}
						</span>
					</div>

					<button
						type="button"
						onClick={handleLogout}
						title="Keluar dari akun"
						aria-label="Keluar dari akun"
						className="p-2 text-slate-400 hover:text-rose-600 rounded-lg transition-colors hover:bg-slate-100 cursor-pointer"
					>
						<LogOut className="w-4 h-4" />
					</button>
				</div>
			</div>

			{/* Create / Manage Organization Modal */}
			<Modal
				isOpen={isOrgModalOpen}
				onClose={() => setIsOrgModalOpen(false)}
				title={currentOrg ? "Kelola Organisasi" : "Buat Organisasi Baru"}
			>
				<div className="space-y-4">
					<p className="text-xs text-slate-600">
						Setiap organisasi memiliki kuota, API key, dan tim yang terisolasi.
					</p>

					{currentOrg && (
						<div className="p-3 bg-slate-50 rounded-xl border border-slate-200 flex items-center justify-between">
							<div>
								<p className="text-xs font-bold text-slate-900">
									{currentOrg.name}
								</p>
								<p className="text-[11px] text-slate-500">
									Paket Saat Ini:{" "}
									<span className="font-semibold text-[#1877F2] uppercase">
										{currentOrg.planCode}
									</span>
								</p>
							</div>
							<span className="text-[10px] font-semibold text-emerald-600 bg-emerald-50 px-2 py-0.5 rounded-full border border-emerald-200">
								Aktif
							</span>
						</div>
					)}

					<form onSubmit={handleCreateOrg} className="space-y-3 pt-2">
						<div>
							<label
								htmlFor="header-new-org-name"
								className="block text-xs font-semibold text-slate-700 mb-1"
							>
								Nama Organisasi / Perusahaan
							</label>
							<div className="relative">
								<Building2 className="w-4 h-4 text-slate-400 absolute left-3 top-2.5" />
								<input
									id="header-new-org-name"
									type="text"
									placeholder="misal: PT Fintech Nusantara"
									value={newOrgName}
									onChange={(e) => setNewOrgName(e.target.value)}
									className="w-full pl-9 pr-3 py-2 text-xs border border-slate-300 rounded-lg focus:outline-hidden focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/20"
								/>
							</div>
						</div>

						<div>
							<label
								htmlFor="header-org-plan"
								className="block text-xs font-semibold text-slate-700 mb-1"
							>
								Paket Berlangganan
							</label>
							<select
								id="header-org-plan"
								value={newOrgPlan}
								onChange={(e) => setNewOrgPlan(e.target.value)}
								className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg bg-white focus:outline-hidden focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/20"
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
								className="px-3 py-2 text-xs font-medium text-slate-600 hover:text-slate-800 bg-white border border-slate-200 rounded-lg hover:bg-slate-50 transition-colors cursor-pointer"
							>
								Batal
							</button>
							<button
								type="submit"
								disabled={!newOrgName.trim() || isCreatingOrg}
								className="px-4 py-2 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] disabled:opacity-50 rounded-lg transition-colors cursor-pointer"
							>
								{currentOrg
									? "Buat Organisasi Baru"
									: "Buat Organisasi & Lanjutkan"}
							</button>
						</div>
					</form>
				</div>
			</Modal>
		</header>
	);
};
