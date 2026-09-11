import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Badge } from './Badge';

describe('Badge', () => {
  it('renders with children and applies variant styles', () => {
    const { rerender } = render(<Badge variant="success">Active</Badge>);
    expect(screen.getByText('Active')).toBeDefined();
    expect(screen.getByText('Active').className).toContain('text-emerald-700');

    rerender(<Badge variant="danger">Revoked</Badge>);
    expect(screen.getByText('Revoked').className).toContain('text-rose-700');

    rerender(<Badge variant="info">Live</Badge>);
    expect(screen.getByText('Live').className).toContain('text-[#1877F2]');
  });
});
