import {
	BarChart3,
	BookOpen,
	KeyRound,
	LayoutDashboard,
	ListFilter,
	Radio,
	Settings,
	ShieldCheck,
} from "lucide-react";
import type React from "react";
import { useAuth } from "../../context/AuthContext";

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
}

export const Sidebar: React.FC<SidebarProps> = ({
	currentPage,
	onNavigate,
	isHealthy,
}) => {
	const { environment, apiKey } = useAuth();

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
		<aside className="w-64 bg-white border-r border-slate-200 flex flex-col shrink-0 h-screen sticky top-0 select-none">
			{/* Brand Header */}
			<div className="h-16 flex items-center px-6 border-b border-slate-100 gap-3">
				<div className="w-9 h-9 rounded-lg bg-[#1877F2] flex items-center justify-center text-white shadow-xs">
					<ShieldCheck className="w-5 h-5" />
				</div>
				<div>
					<div className="flex items-center gap-1.5">
						<span className="font-bold tracking-tight text-slate-900 text-base">
							NusaID
						</span>
						<span className="text-[10px] font-semibold uppercase px-1.5 py-0.2 bg-[#E7F3FF] text-[#1877F2] rounded border border-[#C3DCFC]">
							Portal
						</span>
					</div>
					<p className="text-xs text-slate-500 font-medium">KTP OCR Service</p>
				</div>
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
							type="button"
							onClick={() => onNavigate(item.id)}
							className={`w-full flex items-center gap-3 px-3 py-2 text-sm font-medium rounded-lg transition-colors ${
								isActive
									? "bg-[#E7F3FF] text-[#1877F2] font-semibold"
									: "text-slate-600 hover:text-slate-900 hover:bg-slate-50"
							}`}
						>
							<Icon
								className={`w-4 h-4 ${isActive ? "text-[#1877F2]" : "text-slate-400"}`}
							/>
							<span>{item.label}</span>
						</button>
					);
				})}
			</nav>

			{/* Active Session Info at Bottom */}
			<div className="p-4 border-t border-slate-100 bg-slate-50/50">
				<div className="text-xs text-slate-500">
					<div className="flex items-center gap-1.5 mb-1">
						<Radio className="w-3.5 h-3.5 text-[#1877F2]" />
						<span className="font-semibold text-slate-700">Active Tenant</span>
					</div>
					<p className="truncate font-mono text-[11px] text-slate-600">
						{apiKey ? `${apiKey.slice(0, 14)}••••` : "Default Workspace"}
					</p>
				</div>
			</div>
		</aside>
	);
};
