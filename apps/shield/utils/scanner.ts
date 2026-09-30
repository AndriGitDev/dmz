export const trainedScanner = import.meta.env.VITE_DMZ_TRAINED_SCANNER === '1';
export type NameSpan = { start: number; end: number };
export function scanNames(text: string): Promise<NameSpan[]> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('Scanner deadline')), 125000);
    chrome.runtime
      .sendMessage({ type: 'dmz:scan-names', text })
      .then((response) => {
        if (response?.error || !Array.isArray(response?.spans) || response.spans.length > 512)
          throw new Error('Scanner failed');
        const length = new TextEncoder().encode(text).length;
        let end = 0;
        for (const span of response.spans) {
          if (
            !Number.isInteger(span.start) ||
            !Number.isInteger(span.end) ||
            span.start < end ||
            span.end <= span.start ||
            span.end > length
          )
            throw new Error('Invalid spans');
          end = span.end;
        }
        resolve(response.spans);
      })
      .catch(reject)
      .finally(() => clearTimeout(timer));
  });
}
