/// <reference types="vitest" />

import { readFileSync } from "node:fs";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const pkg = JSON.parse(
	readFileSync(new URL("./package.json", import.meta.url), "utf-8"),
);

// https://vitejs.dev/config/
export default defineConfig({
	define: {
		__APP_VERSION__: JSON.stringify(pkg.version),
	},
	plugins: [react(), tailwindcss()],
	server: {
		port: 3000,
		proxy: {
			"/api": {
				target: "http://localhost:8080",
				changeOrigin: true,
			},
			"/health": {
				target: "http://localhost:8080",
				changeOrigin: true,
			},
		},
	},
	test: {
		environment: "jsdom",
		globals: true,
	},
});
