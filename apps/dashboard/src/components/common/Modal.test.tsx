import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Modal } from './Modal';

describe('Modal', () => {
  it('does not render when isOpen is false', () => {
    render(
      <Modal isOpen={false} onClose={() => {}} title="Test Dialog">
        Content
      </Modal>
    );
    expect(screen.queryByText('Test Dialog')).toBeNull();
  });

  it('renders title, content, footer and handles close clicks', () => {
    const handleClose = vi.fn();
    render(
      <Modal
        isOpen={true}
        onClose={handleClose}
        title="Test Dialog"
        footer={<button type="button">Submit</button>}
      >
        <p>Modal body content</p>
      </Modal>
    );

    expect(screen.getByText('Test Dialog')).toBeDefined();
    expect(screen.getByText('Modal body content')).toBeDefined();
    expect(screen.getByText('Submit')).toBeDefined();

    // Close button
    const closeBtn = screen.getByLabelText('Close dialog');
    fireEvent.click(closeBtn);
    expect(handleClose).toHaveBeenCalledTimes(1);

    // Escape key
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(handleClose).toHaveBeenCalledTimes(2);
  });
});
