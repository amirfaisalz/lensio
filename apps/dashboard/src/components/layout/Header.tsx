import { LogOut, Menu, Moon, Sun } from "lucide-react";
import type React from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../context/AuthContext";
import { useTheme } from "../../context/ThemeContext";

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
	const { theme, toggleTheme } = useTheme();

	const handleLogout = () => {
		logout();
		navigate("/login");
	};

	// Determine display name / email
	const userIdentifier =
		oidcUser?.email || (apiKey ? "API Key Connected" : "Pengguna");
	const userInitial = userIdentifier.charAt(0).toUpperCase();

	return (
		<header className="h-16 bg-white dark:bg-slate-900 border-b border-slate-200 dark:border-white/10 px-4 sm:px-6 lg:px-8 flex items-center justify-between sticky top-0 z-30">
			{/* Left: Hamburger (mobile/tablet) + Page Title */}
			<div className="flex items-center gap-2.5 sm:gap-3 min-w-0 mr-2">
				<button
					type="button"
					onClick={onOpenMobileNav}
					aria-label="Buka menu navigasi"
					className="p-2 -ml-1 text-slate-600 hover:text-slate-900 hover:bg-slate-100 dark:text-slate-300 dark:hover:text-white dark:hover:bg-white/10 rounded-lg lg:hidden cursor-pointer shrink-0 transition-colors"
				>
					<Menu className="w-5 h-5" />
				</button>
				<div className="min-w-0">
					<h1 className="text-base sm:text-lg font-bold text-slate-900 dark:text-white tracking-tight truncate">
						{title}
					</h1>
					{subtitle && (
						<p className="text-xs text-slate-500 hidden sm:block truncate">
							{subtitle}
						</p>
					)}
				</div>
			</div>

			{/* Right: User Profile & Logout */}
			<div className="flex items-center gap-1.5 sm:gap-2 shrink-0">
				<div className="flex items-center gap-2 px-2.5 py-1 rounded-lg bg-slate-50 dark:bg-white/5 border border-slate-200 dark:border-white/10 text-slate-700 dark:text-slate-200">
					<div className="w-6 h-6 rounded-full bg-[#1877F2] text-white flex items-center justify-center text-[11px] font-bold shrink-0">
						{userInitial}
					</div>
					<span className="text-xs font-medium max-w-[160px] sm:max-w-[220px] truncate">
						{userIdentifier}
					</span>
				</div>

				<button
					type="button"
					onClick={toggleTheme}
					title={
						theme === "dark" ? "Aktifkan mode terang" : "Aktifkan mode gelap"
					}
					aria-label={
						theme === "dark" ? "Aktifkan mode terang" : "Aktifkan mode gelap"
					}
					className="p-2 text-slate-400 hover:text-slate-700 dark:hover:text-amber-300 rounded-lg transition-colors hover:bg-slate-100 dark:hover:bg-white/10 cursor-pointer"
				>
					{theme === "dark" ? (
						<Sun className="w-4 h-4" />
					) : (
						<Moon className="w-4 h-4" />
					)}
				</button>

				<button
					type="button"
					onClick={handleLogout}
					title="Keluar dari akun"
					aria-label="Keluar dari akun"
					className="p-2 text-slate-400 hover:text-rose-600 rounded-lg transition-colors hover:bg-slate-100 dark:hover:bg-white/10 cursor-pointer"
				>
					<LogOut className="w-4 h-4" />
				</button>
			</div>
		</header>
	);
};
