import { LogOut, Menu } from "lucide-react";
import type React from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../context/AuthContext";

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
	const { oidcUser, apiKey, logout } = useAuth();

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

			{/* Right: User Profile & Logout */}
			<div className="flex items-center gap-1.5 sm:gap-2 shrink-0">
				<div className="flex items-center gap-2 px-2.5 py-1 rounded-lg bg-slate-50 border border-slate-200 text-slate-700">
					<div className="w-6 h-6 rounded-full bg-[#1877F2] text-white flex items-center justify-center text-[11px] font-bold shrink-0">
						{userInitial}
					</div>
					<span className="text-xs font-medium max-w-[160px] sm:max-w-[220px] truncate">
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
		</header>
	);
};
