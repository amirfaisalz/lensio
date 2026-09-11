import { X } from "lucide-react";
import type React from "react";
import { useEffect } from "react";

interface ModalProps {
	isOpen: boolean;
	onClose: () => void;
	title: string;
	children: React.ReactNode;
	footer?: React.ReactNode;
	maxWidth?: "sm" | "md" | "lg" | "xl" | "2xl";
}

export const Modal: React.FC<ModalProps> = ({
	isOpen,
	onClose,
	title,
	children,
	footer,
	maxWidth = "md",
}) => {
	useEffect(() => {
		const handleKeyDown = (e: KeyboardEvent) => {
			if (e.key === "Escape" && isOpen) {
				onClose();
			}
		};
		if (isOpen) {
			document.body.style.overflow = "hidden";
			window.addEventListener("keydown", handleKeyDown);
		}
		return () => {
			document.body.style.overflow = "";
			window.removeEventListener("keydown", handleKeyDown);
		};
	}, [isOpen, onClose]);

	if (!isOpen) return null;

	const maxWidthClasses = {
		sm: "max-w-sm",
		md: "max-w-md",
		lg: "max-w-lg",
		xl: "max-w-xl",
		"2xl": "max-w-2xl",
	};

	return (
		<div
			role="dialog"
			aria-modal="true"
			aria-labelledby="modal-title"
			className="fixed inset-0 z-50 flex items-center justify-center p-4 animate-in fade-in duration-150"
		>
			<button
				type="button"
				tabIndex={-1}
				aria-label="Close background overlay"
				className="fixed inset-0 bg-black/40 backdrop-blur-xs w-full h-full border-0 cursor-default"
				onClick={onClose}
			/>
			<div
				className={`relative z-10 w-full ${maxWidthClasses[maxWidth]} bg-white rounded-xl shadow-xl border border-slate-200 overflow-hidden flex flex-col max-h-[90vh]`}
			>
				<div className="flex items-center justify-between px-6 py-4 border-b border-slate-100">
					<h3
						id="modal-title"
						className="text-base font-semibold text-slate-900"
					>
						{title}
					</h3>
					<button
						type="button"
						onClick={onClose}
						aria-label="Close dialog"
						className="text-slate-400 hover:text-slate-600 p-1 rounded-md transition-colors hover:bg-slate-100 focus-visible:ring-2 focus-visible:ring-[#1877F2]"
					>
						<X className="w-5 h-5" />
					</button>
				</div>

				<div className="px-6 py-5 overflow-y-auto text-sm text-slate-700 space-y-4">
					{children}
				</div>

				{footer && (
					<div className="flex items-center justify-end gap-3 px-6 py-3 bg-slate-50 border-t border-slate-100">
						{footer}
					</div>
				)}
			</div>
		</div>
	);
};
