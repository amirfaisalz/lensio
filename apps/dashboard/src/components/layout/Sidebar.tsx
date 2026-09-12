import {
	BarChart3,
	BookOpen,
	Building2,
	Check,
	ChevronsUpDown,
	KeyRound,
	LayoutDashboard,
	ListFilter,
	Plus,
	Settings,
	ShieldCheck,
	X,
} from "lucide-react";
import type React from "react";
import { useEffect, useRef, useState } from "react";
import { useAuth } from "../../context/AuthContext";
import { Modal } from "../common/Modal";

export type NavigationPage =
	| "overview"
	| "keys"
	| "usage"
	| "requests"
	| "docs"
	| "account";

interface SidebarProps {
	currentPage: NavigationPage;
	onNavigate: (page: NavigationPage) => void;
	isHealthy: boolean;
	isOpenMobile?: boolean;
	onCloseMobile?: () => void;
}

export const Sidebar: React.FC<SidebarProps> = ({
	currentPage,
	onNavigate,
	isHealthy,
	isOpenMobile = false,
	onCloseMobile,
}) => {
	const {
		environment,
		apiKey,
		currentOrg,
		organizations,
		switchOrganization,
		createOrganization,
		oidcUser,
	} = useAuth();

	const [isOrgDropdownOpen, setIsOrgDropdownOpen] = useState(false);
	const [isCreateOrgModalOpen, setIsCreateOrgModalOpen] = useState(false);
	const [newOrgName, setNewOrgName] = useState("");
	const [newOrgPlan, setNewOrgPlan] = useState("free");
	const [isCreatingOrg, setIsCreatingOrg] = useState(false);

	const dropdownRef = useRef<HTMLDivElement>(null);

	useEffect(() => {
		const handleKeyDown = (e: KeyboardEvent) => {
			if (e.key === "Escape") {
				if (isOrgDropdownOpen) {
					setIsOrgDropdownOpen(false);
				} else if (isOpenMobile && onCloseMobile) {
					onCloseMobile();
				}
			}
		};
		if (isOpenMobile) {
			document.body.style.overflow = "hidden";
			window.addEventListener("keydown", handleKeyDown);
		}
		window.addEventListener("keydown", handleKeyDown);
		return () => {
			document.body.style.overflow = "";
			window.removeEventListener("keydown", handleKeyDown);
		};
	}, [isOpenMobile, onCloseMobile, isOrgDropdownOpen]);

	useEffect(() => {
		const handleClickOutside = (e: MouseEvent) => {
			if (
				dropdownRef.current &&
				!dropdownRef.current.contains(e.target as Node)
			) {
				setIsOrgDropdownOpen(false);
			}
		};
		if (isOrgDropdownOpen) {
			document.addEventListener("mousedown", handleClickOutside);
		}
		return () => {
			document.removeEventListener("mousedown", handleClickOutside);
		};
	}, [isOrgDropdownOpen]);

	const handleCreateOrg = async (e: React.FormEvent) => {
		e.preventDefault();
		if (!newOrgName.trim()) return;

		try {
			setIsCreatingOrg(true);
			await createOrganization(newOrgName.trim(), newOrgPlan);
			setNewOrgName("");
			setIsCreateOrgModalOpen(false);
			setIsOrgDropdownOpen(false);
		} finally {
			setIsCreatingOrg(false);
		}
	};

	const navItems: {
		id: NavigationPage;
		label: string;
		icon: React.FC<{ className?: string }>;
	}[] = [
		{ id: "overview", label: "Overview", icon: LayoutDashboard },
		{ id: "keys", label: "API Keys", icon: KeyRound },
		{ id: "usage", label: "Usage & Analytics", icon: BarChart3 },
		{ id: "requests", label: "Requests Explorer", icon: ListFilter },
		{ id: "docs", label: "API Documentation", icon: BookOpen },
		{ id: "account", label: "Account & Settings", icon: Settings },
	];

	return (
		<>
			{/* Backdrop Overlay for Mobile Drawer */}
			{isOpenMobile && (
				<button
					type="button"
					aria-label="Tutup menu navigasi"
					onClick={onCloseMobile}
					className="fixed inset-0 bg-slate-900/50 backdrop-blur-xs z-40 lg:hidden cursor-default transition-opacity animate-in fade-in duration-200"
				/>
			)}

			{/* Sidebar / Slide-Over Drawer Container */}
			<aside
				className={`fixed inset-y-0 left-0 z-50 w-72 sm:w-64 bg-white border-r border-slate-200 flex flex-col shrink-0 h-screen transition-transform duration-300 ease-in-out select-none lg:sticky lg:top-0 lg:z-30 lg:h-screen lg:translate-x-0 ${
					isOpenMobile ? "translate-x-0 shadow-2xl" : "-translate-x-full"
				}`}
			>
				{/* Brand Header */}
				<div className="h-16 flex items-center justify-between px-6 border-b border-slate-100 gap-3">
					<div className="flex items-center gap-3">
						<div className="w-9 h-9 rounded-lg bg-[#1877F2] flex items-center justify-center text-white shadow-xs">
							<ShieldCheck className="w-5 h-5" />
						</div>
						<div>
							<div className="flex items-center gap-1.5">
								<span className="font-bold tracking-tight text-slate-900 text-base">
									Lensio
								</span>
								<span className="text-[10px] font-semibold uppercase px-1.5 py-0.2 bg-[#E7F3FF] text-[#1877F2] rounded border border-[#C3DCFC]">
									Portal
								</span>
							</div>
							<p className="text-xs text-slate-500 font-medium">
								KTP OCR Service
							</p>
						</div>
					</div>

					{/* Close Drawer Button for Mobile (< lg) */}
					<button
						type="button"
						aria-label="Tutup navigasi"
						onClick={onCloseMobile}
						className="p-1.5 rounded-lg text-slate-400 hover:text-slate-700 hover:bg-slate-100 lg:hidden cursor-pointer transition-colors"
					>
						<X className="w-5 h-5" />
					</button>
				</div>

				{/* Environment & Connection status bar */}
				<div className="px-4 pt-4 pb-2">
					<div className="bg-slate-50 border border-slate-200 rounded-lg p-2.5 flex items-center justify-between text-xs">
						<div className="flex items-center gap-2">
							<span
								className={`w-2 h-2 rounded-full ${
									isHealthy
										? "bg-emerald-500 ring-2 ring-emerald-100"
										: "bg-rose-500 ring-2 ring-rose-100"
								}`}
							/>
							<span className="font-medium text-slate-700">
								{isHealthy ? "API Online" : "API Connecting"}
							</span>
						</div>
						<span
							className={`font-semibold uppercase text-[10px] px-2 py-0.5 rounded ${
								environment === "live"
									? "bg-emerald-100 text-emerald-800"
									: "bg-amber-100 text-amber-800"
							}`}
						>
							{environment}
						</span>
					</div>
				</div>

				{/* Main Navigation */}
				<nav className="flex-1 px-3 py-3 space-y-1 overflow-y-auto">
					{navItems.map((item) => {
						const Icon = item.icon;
						const isActive = currentPage === item.id;
						return (
							<button
								key={item.id}
								data-page={item.id}
								type="button"
								onClick={() => {
									onNavigate(item.id);
									onCloseMobile?.();
								}}
								className={`w-full flex items-center gap-3 px-3 py-2.5 sm:py-2 text-sm font-medium rounded-lg transition-colors cursor-pointer ${
									isActive
										? "bg-[#E7F3FF] text-[#1877F2] font-semibold"
										: "text-slate-600 hover:text-slate-900 hover:bg-slate-50"
								}`}
							>
								<Icon
									className={`w-4 h-4 shrink-0 ${isActive ? "text-[#1877F2]" : "text-slate-400"}`}
								/>
								<span>{item.label}</span>
							</button>
						);
					})}
				</nav>

				{/* Organization Switcher Dropdown at Bottom */}
				<div
					ref={dropdownRef}
					className="p-3 border-t border-slate-100 bg-slate-50/70 relative"
				>
					{/* Dropdown Popover Menu */}
					{isOrgDropdownOpen && (
						<div className="absolute bottom-full mb-2 left-3 right-3 bg-white rounded-xl shadow-xl border border-slate-200 py-1.5 z-50 animate-in fade-in zoom-in-95 duration-150">
							<div className="px-3 py-1.5 border-b border-slate-100 flex items-center justify-between">
								<span className="text-[11px] font-semibold text-slate-500 uppercase tracking-wider">
									Organisasi
								</span>
								<span className="text-[10px] bg-slate-100 text-slate-600 px-1.5 py-0.2 rounded-full font-medium">
									{organizations.length}
								</span>
							</div>

							<div className="max-h-52 overflow-y-auto py-1">
								{organizations.map((org) => {
									const isSelected = currentOrg?.id === org.id;
									return (
										<button
											key={org.id}
											type="button"
											onClick={() => {
												switchOrganization(org);
												setIsOrgDropdownOpen(false);
											}}
											className={`w-full flex items-center justify-between px-3 py-2 text-left text-xs transition-colors cursor-pointer ${
												isSelected
													? "bg-[#E7F3FF] text-[#1877F2] font-semibold"
													: "text-slate-700 hover:bg-slate-50"
											}`}
										>
											<div className="min-w-0 pr-2">
												<p className="truncate text-xs font-semibold">
													{org.name}
												</p>
												<span className="text-[10px] uppercase font-mono px-1 py-0.2 rounded bg-slate-100 text-slate-600">
													{org.planCode}
												</span>
											</div>
											{isSelected && (
												<Check className="w-4 h-4 text-[#1877F2] shrink-0 ml-1" />
											)}
										</button>
									);
								})}
								{organizations.length === 0 && (
									<div className="px-3 py-2 text-xs text-slate-400 italic text-center">
										Belum ada organisasi
									</div>
								)}
							</div>

							<div className="pt-1 border-t border-slate-100 px-1">
								<button
									type="button"
									onClick={() => {
										setIsOrgDropdownOpen(false);
										setIsCreateOrgModalOpen(true);
									}}
									className="w-full flex items-center gap-2 px-2.5 py-1.5 text-xs font-semibold text-[#1877F2] hover:bg-[#E7F3FF] rounded-lg transition-colors cursor-pointer"
								>
									<Plus className="w-3.5 h-3.5" />
									<span>Buat Organisasi Baru</span>
								</button>
							</div>
						</div>
					)}

					{/* Trigger Button */}
					<button
						type="button"
						onClick={() => setIsOrgDropdownOpen((prev) => !prev)}
						aria-expanded={isOrgDropdownOpen}
						aria-label="Ganti organisasi"
						className="w-full p-2.5 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 text-left transition-colors cursor-pointer flex items-center justify-between gap-2 shadow-xs group"
					>
						<div className="flex items-center gap-2.5 min-w-0">
							<div className="w-8 h-8 rounded-lg bg-[#E7F3FF] text-[#1877F2] flex items-center justify-center shrink-0">
								<Building2 className="w-4 h-4" />
							</div>
							<div className="min-w-0 flex-1">
								<div className="flex items-center gap-1.5">
									<span className="truncate font-bold text-xs text-slate-900">
										{currentOrg?.name || "Pilih Organisasi"}
									</span>
									{currentOrg?.planCode && (
										<span className="text-[9px] font-bold uppercase px-1.5 py-0.2 rounded bg-slate-100 text-slate-600 shrink-0">
											{currentOrg.planCode}
										</span>
									)}
								</div>
								<p className="truncate text-[10px] text-slate-500 font-mono mt-0.5">
									{oidcUser?.email ||
										(apiKey ? `${apiKey.slice(0, 10)}••••` : "dev@lensio.dev")}
								</p>
							</div>
						</div>
						<ChevronsUpDown className="w-4 h-4 text-slate-400 group-hover:text-slate-600 shrink-0 ml-1" />
					</button>
				</div>
			</aside>

			{/* Create Organization Modal */}
			<Modal
				isOpen={isCreateOrgModalOpen}
				onClose={() => setIsCreateOrgModalOpen(false)}
				title="Buat Organisasi Baru"
			>
				<form onSubmit={handleCreateOrg} className="space-y-3">
					<p className="text-xs text-slate-600">
						Setiap organisasi memiliki kuota, API key, dan tim yang terisolasi.
					</p>

					<div>
						<label
							htmlFor="sidebar-new-org-name"
							className="block text-xs font-semibold text-slate-700 mb-1"
						>
							Nama Organisasi / Perusahaan
						</label>
						<div className="relative">
							<Building2 className="w-4 h-4 text-slate-400 absolute left-3 top-2.5" />
							<input
								id="sidebar-new-org-name"
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
							htmlFor="sidebar-org-plan"
							className="block text-xs font-semibold text-slate-700 mb-1"
						>
							Paket Berlangganan
						</label>
						<select
							id="sidebar-org-plan"
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
							onClick={() => setIsCreateOrgModalOpen(false)}
							className="px-3 py-2 text-xs font-medium text-slate-600 hover:text-slate-800 bg-white border border-slate-200 rounded-lg hover:bg-slate-50 transition-colors cursor-pointer"
						>
							Batal
						</button>
						<button
							type="submit"
							disabled={!newOrgName.trim() || isCreatingOrg}
							className="px-4 py-2 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] disabled:opacity-50 rounded-lg transition-colors cursor-pointer"
						>
							{isCreatingOrg ? "Membuat..." : "Buat Organisasi & Lanjutkan"}
						</button>
					</div>
				</form>
			</Modal>
		</>
	);
};
