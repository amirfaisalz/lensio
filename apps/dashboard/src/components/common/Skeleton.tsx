import type React from "react";

interface SkeletonProps {
	className?: string;
}

export const Skeleton: React.FC<SkeletonProps> = ({
	className = "h-4 w-full",
}) => {
	return (
		<div
			className={`animate-pulse bg-slate-200/80 dark:bg-white/10 rounded ${className}`}
		/>
	);
};

export const TableSkeleton: React.FC<{ rows?: number; cols?: number }> = ({
	rows = 5,
	cols = 4,
}) => {
	const rowKeys = Array.from(
		{ length: rows },
		(_, i) => `skeleton-row-${i + 1}`,
	);
	const colKeys = Array.from(
		{ length: cols },
		(_, i) => `skeleton-col-${i + 1}`,
	);

	return (
		<div className="space-y-3">
			{rowKeys.map((rowKey) => (
				<div key={rowKey} className="flex gap-4">
					{colKeys.map((colKey, cIdx) => (
						<Skeleton
							key={colKey}
							className={`h-6 ${cIdx === 0 ? "w-1/4" : "flex-1"}`}
						/>
					))}
				</div>
			))}
		</div>
	);
};
