import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import App from './App';

describe('App & Dashboard Navigation', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();

    vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
      const url = String(input);
      if (url.includes('/health')) {
        return { ok: true, json: async () => ({ status: 'ok', timestamp: '2026-09-11T00:00:00Z' }) } as Response;
      }
      if (url.includes('/api/v1/usage/daily')) {
        return { ok: true, json: async () => ({ data: [] }) } as Response;
      }
      if (url.includes('/api/v1/usage/endpoints')) {
        return { ok: true, json: async () => ({ data: [] }) } as Response;
      }
      if (url.includes('/api/v1/usage/records')) {
        return { ok: true, json: async () => ({ data: [], total: 0, limit: 25, offset: 0 }) } as Response;
      }
      if (url.includes('/api/v1/usage')) {
        return {
          ok: true,
          json: async () => ({
            total_requests: 42,
            success_count: 40,
            error_count: 2,
            quota_limit: 1000,
            quota_remaining: 958,
            p95_latency_ms: 180,
            rate_limit_violations: 0,
            billing_cycle_reset: '2026-10-01T00:00:00Z',
          }),
        } as Response;
      }
      if (url.includes('/api/v1/auth/api-keys')) {
        return { ok: true, json: async () => ({ data: [] }) } as Response;
      }
      if (url.includes('/api/v1/account/plan')) {
        return {
          ok: true,
          json: async () => ({
            plan_code: 'free',
            plan_name: 'Free Tier',
            monthly_quota: 100,
            rate_limit_per_minute: 10,
            billing_cycle_reset: '2026-10-01T00:00:00Z',
          }),
        } as Response;
      }
      if (url.includes('/api/v1/account/members')) {
        return { ok: true, json: async () => ({ data: [] }) } as Response;
      }
      if (url.includes('/api/v1/account')) {
        return {
          ok: true,
          json: async () => ({
            organization_id: '00000000-0000-0000-0000-000000000001',
            organization_name: 'Default Organization',
            slug: 'default',
            plan_code: 'free',
            plan_name: 'Free Tier',
            active_keys_count: 1,
            created_at: '2026-09-11T00:00:00Z',
          }),
        } as Response;
      }
      return { ok: true, json: async () => ({}) } as Response;
    });
  });

  it('renders sidebar, header, and navigates across all 6 pages', async () => {
    render(<App />);

    // Initial Overview page
    expect(screen.getByText('NusaID')).toBeDefined();
    await waitFor(() => {
      expect(screen.getByText('System Overview')).toBeDefined();
    });

    // Navigate to API Keys
    fireEvent.click(screen.getByText('API Keys'));
    await waitFor(() => {
      expect(screen.getByText('API Key Management')).toBeDefined();
    });

    // Navigate to Usage & Analytics
    fireEvent.click(screen.getByText('Usage & Analytics'));
    await waitFor(() => {
      expect(screen.getByText('Daily Request Volume')).toBeDefined();
    });

    // Navigate to Requests Explorer
    fireEvent.click(screen.getByText('Requests Explorer'));
    await waitFor(() => {
      expect(screen.getByText('Requests Explorer', { selector: 'h2' })).toBeDefined();
    });

    // Navigate to API Documentation
    fireEvent.click(screen.getByText('API Documentation'));
    await waitFor(() => {
      expect(screen.getByText('Quickstart Integration Snippet')).toBeDefined();
    });

    // Navigate to Account & Settings
    fireEvent.click(screen.getByText('Account & Settings'));
    await waitFor(() => {
      expect(screen.getByText('Organization Profile')).toBeDefined();
    });
  });
});
