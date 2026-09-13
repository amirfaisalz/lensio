import type React from "react";

export type BadgeVariant =
	| "success"
	| "danger"
	| "warning"
	| "info"
	| "neutral";

interface BadgeProps {
	children: React.ReactNode;
	variant?: BadgeVariant;
	className?: string;
}

export const Badge: React.FC<BadgeProps> = ({
	children,
	variant = "neutral",
	className = "",
}) => {
	const variantStyles: Record<BadgeVariant, string> = {
		success:
			"bg-emerald-50 dark:bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 border-emerald-200 dark:border-emerald-500/20",
		danger:
			"bg-rose-50 dark:bg-rose-500/10 text-rose-700 dark:text-rose-300 border-rose-200 dark:border-rose-500/20",
		warning:
			"bg-amber-50 dark:bg-amber-500/10 text-amber-700 dark:text-amber-300 border-amber-200 dark:border-amber-500/20",
		info: "bg-[#E7F3FF] dark:bg-[#1877F2]/15 text-[#1877F2] dark:text-[#7aa9f5] border-[#C3DCFC] dark:border-transparent",
		neutral:
			"bg-slate-100 dark:bg-white/10 text-slate-700 dark:text-slate-300 border-slate-200 dark:border-white/10",
	};

	return (
		<span
			className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] font-semibold tabular-nums ${variantStyles[variant]} ${className}`}
		>
			{children}
		</span>
	);
};
