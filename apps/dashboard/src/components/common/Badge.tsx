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
		success: "bg-emerald-50 text-emerald-700 border-emerald-200",
		danger: "bg-rose-50 text-rose-700 border-rose-200",
		warning: "bg-amber-50 text-amber-700 border-amber-200",
		info: "bg-[#E7F3FF] text-[#1877F2] border-[#C3DCFC]",
		neutral: "bg-slate-100 text-slate-700 border-slate-200",
	};

	return (
		<span
			className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border tabular-nums ${variantStyles[variant]} ${className}`}
		>
			{children}
		</span>
	);
};
