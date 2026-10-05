// Shared annotation validation. No model, network or persistent browser storage.
const ReviewCore = (() => {
  const kinds = ['Person', 'Address', 'Kennitala', 'Email', 'Phone', 'IBAN', 'Card', 'Secret', 'TenantTerm'];
  const encoder = new TextEncoder();
  function require(value, message) { if (!value) throw Error(message); }
  function boundary(text, offset) {
    require(Number.isInteger(offset) && offset >= 0 && offset <= text.length, 'Invalid text boundary');
    const before = text.charCodeAt(offset - 1), after = text.charCodeAt(offset);
    require(!(before >= 0xd800 && before <= 0xdbff && after >= 0xdc00 && after <= 0xdfff), 'Selection splits a Unicode character');
    return encoder.encode(text.slice(0, offset)).length;
  }
  function utf16(text, byteOffset) {
    require(Number.isInteger(byteOffset) && byteOffset >= 0, 'Invalid byte boundary');
    let bytes = 0, units = 0;
    for (const character of text) {
      if (bytes === byteOffset) return units;
      bytes += encoder.encode(character).length; units += character.length;
    }
    require(bytes === byteOffset, 'Annotation splits a UTF-8 character');
    return units;
  }
  function entities(values, text, terms) {
    require(Array.isArray(values), 'Unreviewed labels are not a negative excerpt');
    const result = values.map(value => {
      require(value && typeof value === 'object' && kinds.includes(value.kind), 'Unknown entity type');
      const {start, end, kind} = value;
      require(Number.isInteger(start) && Number.isInteger(end) && start >= 0 && end > start, 'Invalid entity bounds');
      const selected = text.slice(utf16(text, start), utf16(text, end));
      require(selected.trim(), 'Select non-whitespace entity text');
      require(kind !== 'TenantTerm' || terms.includes(selected), 'TenantTerm must exactly match a configured term');
      return {start, end, kind};
    }).sort((a,b) => a.start-b.start || a.end-b.end);
    require(result.every((value,i) => !i || result[i-1].end <= value.start), 'Entity spans overlap; choose one semantic type');
    return result;
  }
  function formRow(row, item, terms, complete = false) {
    require(row && row.id === item.id && row.text_sha256 === item.text_sha256, 'Review text identity differs from packet');
    require(typeof row.reviewed === 'boolean' && typeof row.extraction_verified === 'boolean', 'Invalid review status');
    require(row.reviewer === null || typeof row.reviewer === 'string', 'Invalid reviewer code');
    require(Array.isArray(row.issues) && row.issues.every(issue => typeof issue === 'string'), 'Invalid issue notes');
    require(row.exclusion_reason === null || (typeof row.exclusion_reason === 'string' && row.exclusion_reason.trim()), 'Exclusion needs a reason');
    if (row.exclusion_reason !== null) require(row.entities === null, 'An excluded excerpt cannot be labelled negative');
    const validated = row.entities === null ? null : entities(row.entities, item.text, terms);
    if (complete || row.reviewed) {
      require(row.reviewed && row.reviewer?.trim(), 'Explicit review and reviewer code required');
      if (row.exclusion_reason === null) require(row.extraction_verified && validated !== null, 'Verify extraction and review every supported type');
    }
    return {id: item.id, text_sha256: item.text_sha256, entities: validated, reviewed: row.reviewed,
      extraction_verified: row.extraction_verified, exclusion_reason: row.exclusion_reason,
      reviewer: row.reviewer, issues: [...row.issues]};
  }
  function progress(value, packet) {
    require(value?.schema === 1 && value.packet_sha256 === packet.packet_sha256 && value.slot === packet.slot, 'Progress belongs to a different packet or reviewer slot');
    require(typeof value.reviewer === 'string' && Array.isArray(value.rows) && value.rows.length === packet.items.length, 'Incomplete progress file');
    const rows = new Map(value.rows.map(row => [row?.id, row]));
    require(rows.size === packet.items.length, 'Duplicate review excerpt');
    const drafts = value.draft_exclusions ?? packet.items.map(() => '');
    require(Array.isArray(drafts) && drafts.length === packet.items.length && drafts.every(text => typeof text === 'string'), 'Invalid draft exclusion notes');
    return {reviewer: value.reviewer, draft_exclusions: [...drafts], rows: packet.items.map(item => formRow(rows.get(item.id), item, packet.terms))};
  }
  return {kinds, require, boundary, utf16, entities, formRow, progress};
})();
