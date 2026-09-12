import { describe, expect, it } from "vitest";
import { APP_VERSION } from "./version";

describe("Frontend Versioning Subsystem", () => {
	it("exports a valid non-empty semver version string", () => {
		expect(typeof APP_VERSION).toBe("string");
		expect(APP_VERSION.length).toBeGreaterThan(0);
		// Matches semver like x.y.z or commit-sha/tag
		expect(APP_VERSION).toMatch(/^\d+\.\d+\.\d+/);
	});
});
