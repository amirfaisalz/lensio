// Single source of truth for dashboard frontend application version
declare const __APP_VERSION__: string | undefined;

export const APP_VERSION: string =
	(typeof __APP_VERSION__ !== "undefined" ? __APP_VERSION__ : undefined) ||
	import.meta.env?.VITE_APP_VERSION ||
	"1.0.0";
