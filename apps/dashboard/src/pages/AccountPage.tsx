import React, { useState, useEffect } from 'react';
import {
  Building2,
  CreditCard,
  Users,
  RefreshCw,
  Check,
} from 'lucide-react';
import { api } from '../services/api';
import type { OrganizationDetails, PlanDetails, UserMember } from '../types/api';
import { Badge } from '../components/common/Badge';
import { Skeleton } from '../components/common/Skeleton';

export const AccountPage: React.FC = () => {
  const [org, setOrg] = useState<OrganizationDetails | null>(null);
  const [plan, setPlan] = useState<PlanDetails | null>(null);
  const [members, setMembers] = useState<UserMember[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Plan update state
  const [selectedPlan, setSelectedPlan] = useState<string>('free');
  const [isUpdatingPlan, setIsUpdatingPlan] = useState(false);
  const [planUpdateSuccess, setPlanUpdateSuccess] = useState(false);

  const loadData = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const [orgRes, planRes, membersRes] = await Promise.all([
        api.fetchAccount(),
        api.fetchAccountPlan(),
        api.fetchAccountMembers(),
      ]);
      setOrg(orgRes);
      setPlan(planRes);
      setSelectedPlan(planRes.plan_code);
      setMembers(membersRes);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load account information');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleUpdatePlan = async () => {
    if (!plan || selectedPlan === plan.plan_code) return;

    try {
      setIsUpdatingPlan(true);
      setPlanUpdateSuccess(false);
      await api.updateAccountPlan(selectedPlan);
      setPlanUpdateSuccess(true);
      await loadData();
      setTimeout(() => setPlanUpdateSuccess(false), 3000);
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to update subscription plan');
    } finally {
      setIsUpdatingPlan(false);
    }
  };

  const planOptions = [
    {
      code: 'free',
      name: 'Free Tier',
      quota: '100 requests/mo',
      rateLimit: '10 req/min',
      price: '$0 / mo',
      desc: 'Ideal for testing and development integration.',
    },
    {
      code: 'starter',
      name: 'Starter Tier',
      quota: '1,000 requests/mo',
      rateLimit: '30 req/min',
      price: '$29 / mo',
      desc: 'For early-stage products and growing MVPs.',
    },
    {
      code: 'pro',
      name: 'Pro Tier',
      quota: '10,000 requests/mo',
      rateLimit: '100 req/min',
      price: '$199 / mo',
      desc: 'Production scale with high concurrency limits.',
    },
  ];

  return (
    <div className="space-y-8 animate-in fade-in duration-200">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-bold text-slate-900 tracking-tight">Account & Settings</h2>
          <p className="text-xs text-slate-500">
            Tenant organization profile, subscription tier, and registered team members.
          </p>
        </div>
        <button
          type="button"
          onClick={loadData}
          disabled={isLoading}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-slate-700 bg-white border border-slate-200 rounded-lg hover:bg-slate-50 transition-colors"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${isLoading ? 'animate-spin' : ''}`} />
          <span>Refresh</span>
        </button>
      </div>

      {error && (
        <div className="p-4 bg-rose-50 border border-rose-200 rounded-xl text-xs text-rose-700 flex items-center justify-between">
          <span>{error}</span>
          <button type="button" onClick={loadData} className="font-semibold underline">Retry</button>
        </div>
      )}

      {/* Organization Profile Card */}
      <div className="bg-white p-6 rounded-xl border border-slate-200 shadow-xs">
        <div className="flex items-center gap-2 mb-4">
          <Building2 className="w-4 h-4 text-[#1877F2]" />
          <h3 className="text-sm font-semibold text-slate-900">Organization Profile</h3>
        </div>

        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-48" />
            <Skeleton className="h-4 w-64" />
          </div>
        ) : org ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 text-xs">
            <div className="p-3 bg-slate-50 rounded-lg border border-slate-100">
              <span className="text-slate-400 block text-[10px] uppercase font-semibold">Organization Name</span>
              <span className="font-bold text-slate-900 text-sm">{org.organization_name}</span>
            </div>
            <div className="p-3 bg-slate-50 rounded-lg border border-slate-100">
              <span className="text-slate-400 block text-[10px] uppercase font-semibold">Tenant Slug</span>
              <span className="font-mono text-slate-700">{org.slug}</span>
            </div>
            <div className="p-3 bg-slate-50 rounded-lg border border-slate-100">
              <span className="text-slate-400 block text-[10px] uppercase font-semibold">Organization ID</span>
              <span className="font-mono text-[11px] text-slate-700">{org.organization_id}</span>
            </div>
            <div className="p-3 bg-slate-50 rounded-lg border border-slate-100">
              <span className="text-slate-400 block text-[10px] uppercase font-semibold">Active API Keys</span>
              <span className="font-bold text-[#1877F2] text-sm tabular-nums">{org.active_keys_count} active</span>
            </div>
          </div>
        ) : null}
      </div>

      {/* Subscription Plan Switcher */}
      <div className="bg-white p-6 rounded-xl border border-slate-200 shadow-xs">
        <div className="flex items-center justify-between mb-4">
          <div>
            <div className="flex items-center gap-2">
              <CreditCard className="w-4 h-4 text-[#1877F2]" />
              <h3 className="text-sm font-semibold text-slate-900">Subscription Plan & Quotas</h3>
            </div>
            <p className="text-xs text-slate-500">
              Select an API tier to instantly adjust your monthly quota and per-minute throughput limits.
            </p>
          </div>

          {planUpdateSuccess && (
            <span className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-600 bg-emerald-50 px-2.5 py-1 rounded-md border border-emerald-200">
              <Check className="w-3.5 h-3.5" /> Plan Updated Successfully
            </span>
          )}
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
          {planOptions.map((opt) => {
            const isCurrent = plan?.plan_code === opt.code;
            const isSelected = selectedPlan === opt.code;

            return (
              <div
                key={opt.code}
                onClick={() => setSelectedPlan(opt.code)}
                className={`p-4 rounded-xl border cursor-pointer transition-all ${
                  isSelected
                    ? 'border-[#1877F2] ring-2 ring-[#1877F2]/20 bg-[#E7F3FF]/20'
                    : 'border-slate-200 hover:border-slate-300 bg-white'
                }`}
              >
                <div className="flex items-center justify-between mb-2">
                  <span className="font-bold text-slate-900 text-sm">{opt.name}</span>
                  {isCurrent && (
                    <Badge variant="success">Current Plan</Badge>
                  )}
                </div>

                <div className="text-lg font-bold text-slate-900 mb-1">{opt.price}</div>
                <p className="text-xs text-slate-500 mb-3">{opt.desc}</p>

                <div className="space-y-1 text-xs text-slate-700 border-t border-slate-100 pt-3 tabular-nums">
                  <div className="flex justify-between">
                    <span className="text-slate-500">Monthly Quota:</span>
                    <strong className="text-slate-900">{opt.quota}</strong>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-500">Rate Limit:</span>
                    <strong className="text-slate-900">{opt.rateLimit}</strong>
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        {plan && selectedPlan !== plan.plan_code && (
          <div className="flex items-center justify-end gap-3 pt-4 border-t border-slate-100">
            <button
              type="button"
              onClick={() => setSelectedPlan(plan.plan_code)}
              className="px-4 py-2 text-xs font-medium text-slate-600 hover:text-slate-900"
            >
              Reset
            </button>
            <button
              type="button"
              onClick={handleUpdatePlan}
              disabled={isUpdatingPlan}
              className="px-4 py-2 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] rounded-lg transition-colors shadow-xs"
            >
              {isUpdatingPlan ? 'Updating Plan...' : `Switch to ${selectedPlan.toUpperCase()}`}
            </button>
          </div>
        )}
      </div>

      {/* Team Members List */}
      <div className="bg-white rounded-xl border border-slate-200 shadow-xs overflow-hidden">
        <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Users className="w-4 h-4 text-[#1877F2]" />
            <h3 className="text-sm font-semibold text-slate-900">Organization Members</h3>
          </div>
          <span className="text-xs text-slate-500">{members.length} Member(s)</span>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs text-slate-600">
            <thead className="bg-slate-50 border-b border-slate-200 text-[11px] font-semibold text-slate-500 uppercase tracking-wider">
              <tr>
                <th className="px-6 py-3.5">Name</th>
                <th className="px-6 py-3.5">Email</th>
                <th className="px-6 py-3.5">Role</th>
                <th className="px-6 py-3.5">Joined Date</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 font-medium">
              {isLoading ? (
                <tr>
                  <td colSpan={4} className="px-6 py-8">
                    <Skeleton className="h-6 w-full my-2" />
                  </td>
                </tr>
              ) : members.length === 0 ? (
                <tr>
                  <td colSpan={4} className="px-6 py-8 text-center text-slate-400">
                    No members listed.
                  </td>
                </tr>
              ) : (
                members.map((m) => (
                  <tr key={m.id} className="hover:bg-slate-50/80 transition-colors">
                    <td className="px-6 py-3.5 font-semibold text-slate-900">{m.full_name}</td>
                    <td className="px-6 py-3.5 font-mono text-slate-600">{m.email}</td>
                    <td className="px-6 py-3.5">
                      <Badge variant={m.role === 'owner' ? 'info' : 'neutral'}>
                        {m.role}
                      </Badge>
                    </td>
                    <td className="px-6 py-3.5 text-slate-500 tabular-nums">
                      {new Date(m.created_at).toLocaleDateString(undefined, {
                        month: 'short',
                        day: 'numeric',
                        year: 'numeric',
                      })}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
