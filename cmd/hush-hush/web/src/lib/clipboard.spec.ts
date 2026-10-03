import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createCopier } from './clipboard';

describe('createCopier', () => {
	const writeText = vi.fn();

	beforeEach(() => {
		vi.useFakeTimers();
		writeText.mockReset().mockResolvedValue(undefined);
		vi.stubGlobal('navigator', { clipboard: { writeText } });
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
	});

	it('writes the text and reports the key as copied, then clears it after 1500ms', async () => {
		const onChange = vi.fn();
		const copier = createCopier<string>(onChange);

		await copier.copy('a', 'secret-text');

		expect(writeText).toHaveBeenCalledWith('secret-text');
		expect(onChange).toHaveBeenLastCalledWith('a');

		vi.advanceTimersByTime(1499);
		expect(onChange).toHaveBeenLastCalledWith('a');
		vi.advanceTimersByTime(1);
		expect(onChange).toHaveBeenLastCalledWith(null);
	});

	it('reports nothing as copied when the write rejects', async () => {
		writeText.mockRejectedValue(new Error('blocked'));
		const onChange = vi.fn();

		await createCopier<string>(onChange).copy('a', 'x');

		expect(onChange).not.toHaveBeenCalled();
	});

	it('cancels the first timer when a second copy lands in the window', async () => {
		const onChange = vi.fn();
		const copier = createCopier<string>(onChange);

		await copier.copy('a', 'x');
		vi.advanceTimersByTime(1000);
		await copier.copy('b', 'y');
		vi.advanceTimersByTime(1000);

		expect(onChange).toHaveBeenLastCalledWith('b');
		vi.advanceTimersByTime(500);
		expect(onChange).toHaveBeenLastCalledWith(null);
		expect(onChange.mock.calls.filter(([key]) => key === null)).toHaveLength(1);
	});

	it('stops the pending timer on dispose', async () => {
		const onChange = vi.fn();
		const copier = createCopier<string>(onChange);

		await copier.copy('a', 'x');
		copier.dispose();
		vi.advanceTimersByTime(5000);

		expect(onChange).toHaveBeenCalledTimes(1);
	});
});
