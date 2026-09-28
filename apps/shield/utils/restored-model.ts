// Deliberately no HTML, event handlers, URLs, images or site scripts cross
// into the extension renderer. Only these structural tags and CSS values do.
export const restoredTags = new Set(['div', 'span', 'p', 'br', 'strong', 'b', 'em', 'i', 's', 'u', 'code', 'pre', 'ul', 'ol', 'li', 'blockquote', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6']);
export const restoredStyles = ['color', 'background-color', 'font-family', 'font-size', 'font-weight', 'font-style', 'line-height', 'letter-spacing', 'text-align', 'text-decoration-line', 'white-space', 'margin-top', 'margin-bottom', 'padding-left', 'padding-right', 'list-style-type'];
export type RestoredNode = { text: string } | { tag: string; style: Record<string, string>; children: RestoredNode[] };
