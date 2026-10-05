(() => {
  const $ = id => document.getElementById(id), C = ReviewCore, encoder = new TextEncoder();
  let packet, rows, draftExclusions, current = 0, dirty = false;
  const digest = async text => Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', encoder.encode(text))), b => b.toString(16).padStart(2,'0')).join('');
  const lines = text => text.trim().split('\n').filter(Boolean).map(JSON.parse);
  const item = () => packet.items[current], row = () => rows[current];
  const message = text => { $('message').textContent = text; };
  function action(fn) { return () => { try { fn(); } catch (error) { message(error.message); } }; }
  function changed() { dirty = true; }
  function render() {
    const r = row(), source = item();
    $('excerpt-title').textContent = `${source.id} · ${current + 1} of ${rows.length}`;
    $('source').textContent = `Source: ${source.source_url || 'Ask packet owner for source'} · Stratum: ${source.stratum || 'unspecified'}`;
    $('review-state').textContent = r.reviewed ? `${r.exclusion_reason ? 'Excluded with a recorded reason' : 'Reviewed'} · ${r.reviewer}` : 'Not yet reviewed';
    $('excerpt-select').value = String(current);
    $('reviewer').disabled = false;
    const text = $('excerpt'); text.replaceChildren();
    let position = 0;
    for (const entity of r.entities || []) {
      const start = C.utf16(source.text, entity.start), end = C.utf16(source.text, entity.end);
      text.append(document.createTextNode(source.text.slice(position,start)));
      const mark = document.createElement('mark'); mark.textContent = source.text.slice(start,end); mark.title = entity.kind;
      text.append(mark); position = end;
    }
    text.append(document.createTextNode(source.text.slice(position)));
    $('annotations').replaceChildren();
    (r.entities || []).forEach((entity, index) => {
      const li = document.createElement('li');
      li.append(document.createTextNode(`${entity.kind}: ${source.text.slice(C.utf16(source.text,entity.start), C.utf16(source.text,entity.end))} `));
      const remove = document.createElement('button'); remove.textContent = 'Remove'; remove.disabled = r.reviewed;
      remove.addEventListener('click', action(() => { C.require(!row().reviewed, 'Reopen this excerpt before editing'); row().entities.splice(index,1); changed(); render(); }));
      li.append(remove); $('annotations').append(li);
    });
    $('extraction').checked = r.extraction_verified; $('all-types').checked = r.reviewed && !r.exclusion_reason;
    $('issues').value = r.issues.join('\n'); $('exclusion').value = r.exclusion_reason || draftExclusions[current];
    for (const id of ['add','extraction','all-types','issues','exclusion','reviewed','exclude']) $(id).disabled = r.reviewed;
    $('reopen').disabled = !r.reviewed;
    const reviewed = rows.filter(value => value.reviewed).length;
    $('progress').textContent = `${reviewed} of ${rows.length} reviewed · ${rows.length-reviewed} remaining`;
    $('export').disabled = reviewed !== rows.length;
    $('previous').disabled = current === 0; $('next').disabled = current + 1 === rows.length;
    [...$('excerpt-select').options].forEach((option,i) => { option.textContent = `${packet.items[i].id}${rows[i].reviewed ? (rows[i].exclusion_reason ? ' · excluded' : ' · reviewed') : ''}`; });
    message('');
  }
  function selectionBounds() {
    const selection = window.getSelection(); C.require(selection.rangeCount === 1 && !selection.isCollapsed, 'Select entity text in the original excerpt');
    const range = selection.getRangeAt(0), container = $('excerpt');
    C.require(container.contains(range.startContainer) && container.contains(range.endContainer), 'Select only inside the original excerpt');
    C.require(container.textContent === item().text, 'Rendered text identity changed; reload the packet');
    const prefix = document.createRange(); prefix.selectNodeContents(container); prefix.setEnd(range.startContainer,range.startOffset);
    const start = prefix.toString().length; prefix.setEnd(range.endContainer,range.endOffset);
    return {start: C.boundary(item().text,start), end: C.boundary(item().text,prefix.toString().length)};
  }
  function reviewer() { const value = $('reviewer').value.trim(); C.require(value, 'Enter your reviewer code'); return value; }
  function download(name, text, type) {
    const url = URL.createObjectURL(new Blob([text], {type})), link = document.createElement('a');
    link.href = url; link.download = name; link.click(); setTimeout(() => URL.revokeObjectURL(url),1000); dirty = false;
  }
  $('add').addEventListener('click', action(() => {
    C.require(!row().reviewed, 'Reopen this excerpt before editing');
    const bounds = selectionBounds(); row().entities = C.entities([...(row().entities || []), {...bounds,kind:$('kind').value}],item().text,packet.terms);
    changed(); render(); message('Entity added. Review all types before marking this excerpt complete.');
  }));
  $('extraction').addEventListener('change', () => { row().extraction_verified = $('extraction').checked; changed(); });
  $('issues').addEventListener('input', () => { row().issues = $('issues').value.split('\n').filter(value => value.trim()); changed(); });
  $('reviewer').addEventListener('input', changed);
  $('exclusion').addEventListener('input', () => {draftExclusions[current]=$('exclusion').value;changed();});
  $('reviewed').addEventListener('click', action(() => {
    C.require($('all-types').checked && $('extraction').checked, 'Confirm extraction and review of all nine types');
    rows[current] = C.formRow({...row(), reviewer: reviewer(), reviewed:true, extraction_verified:true, entities:row().entities || [], exclusion_reason:null},item(),packet.terms,true);
    changed(); render(); message('Review recorded. Download progress to keep your work.');
  }));
  $('exclude').addEventListener('click', action(() => {
    const reason = $('exclusion').value.trim(); C.require(reason, 'Explain why the excerpt cannot be reviewed');
    rows[current] = C.formRow({...row(), reviewer:reviewer(), reviewed:true, entities:null, extraction_verified:false, exclusion_reason:reason},item(),packet.terms,true);
    changed(); render(); message('Excluded with a reason; this is not a negative label.');
  }));
  $('reopen').addEventListener('click', action(() => {
    draftExclusions[current] = row().exclusion_reason || draftExclusions[current];
    row().reviewed = false; row().exclusion_reason = null; changed(); render();
  }));
  $('previous').addEventListener('click', () => { if(current) {current--;render();} });
  $('next').addEventListener('click', () => { if(current+1<rows.length) {current++;render();} });
  $('excerpt-select').addEventListener('change', () => {current=Number($('excerpt-select').value);render();});
  $('save-progress').addEventListener('click', action(() => {
    const value = {schema:1, packet_sha256:packet.packet_sha256, slot:packet.slot, reviewer:$('reviewer').value, rows, draft_exclusions:draftExclusions};
    C.progress(value,packet); download(`reviewer-${packet.slot}-progress.json`,JSON.stringify(value,null,2)+'\n','application/json'); message('Progress downloaded. Keep the file private.');
  }));
  $('load-progress').addEventListener('change', async () => {
    try {
      const file = $('load-progress').files[0]; if(!file) return;
      C.require(file.size <= 5*1024*1024, 'Progress file is too large');
      const loaded = C.progress(JSON.parse(await file.text()),packet);
      rows = loaded.rows; draftExclusions=loaded.draft_exclusions; $('reviewer').value = loaded.reviewer; current=0; dirty=false; render(); message('Progress restored after packet, slot and annotation checks.');
    } catch (error) { message(error instanceof SyntaxError ? 'Progress file is not valid JSON' : error.message); }
    finally { $('load-progress').value=''; }
  });
  $('export').addEventListener('click', action(() => {
    const completed = rows.map((value,i) => C.formRow(value,packet.items[i],packet.terms,true));
    download(`reviewer-${packet.slot}.jsonl`,completed.map(value => JSON.stringify(value)).join('\n')+'\n','application/x-ndjson');
    message('Completed reviewer form downloaded. Return it privately for separate adjudication.');
  }));
  window.addEventListener('beforeunload', event => { if(dirty) {event.preventDefault();event.returnValue='';} });
  async function start() {
    const raw = JSON.parse($('packet').textContent);
    C.require(raw.schema === 1 && ['a','b'].includes(raw.slot) && JSON.stringify(raw.scope) === JSON.stringify(C.kinds), 'Unsupported review packet');
    C.require(await digest(raw.index_jsonl) === raw.index_sha256 && await digest(raw.form_jsonl) === raw.form_sha256 && await digest(raw.rubric) === raw.rubric_sha256, 'Packet content hash mismatch');
    C.require(await digest(raw.terms_json) === raw.terms_sha256, 'Tenant dictionary hash mismatch');
    const terms = JSON.parse(raw.terms_json); C.require(Array.isArray(terms) && terms.every(value => typeof value === 'string' && value.trim()) && new Set(terms).size === terms.length, 'Invalid tenant dictionary');
    const items = lines(raw.index_jsonl), initial = lines(raw.form_jsonl);
    C.require(items.length > 0 && items.length === initial.length && new Set(items.map(value=>value.id)).size === items.length, 'Invalid excerpt coverage');
    for (const source of items) {
      const text = raw.texts[source.id]; C.require(typeof text === 'string' && await digest(text) === source.text_sha256, 'Source text hash mismatch'); source.text = text;
    }
    packet = {slot:raw.slot, packet_sha256:raw.packet_sha256, items, terms};
    rows = items.map((source,i) => C.formRow(initial[i],source,terms));
    draftExclusions = items.map(() => '');
    C.require(rows.every(value => !value.reviewed && !value.extraction_verified && value.entities === null && value.reviewer === null && value.exclusion_reason === null && value.issues.length === 0), 'Blind editor requires an original blank review form');
    $('rubric').textContent = raw.rubric;
    for(const kind of C.kinds) {const option=document.createElement('option');option.value=kind;option.textContent=kind;$('kind').append(option);}
    items.forEach((source,i) => {const option=document.createElement('option');option.value=String(i);option.textContent=source.id;$('excerpt-select').append(option);});
    for(const term of terms) {const li=document.createElement('li');li.textContent=term;$('terms').append(li);}
    if(!terms.length) {const li=document.createElement('li');li.textContent='No configured terms.';$('terms').append(li);}
    $('packet-title').textContent = `Reviewer ${raw.slot.toUpperCase()} · ${items.length} excerpts · Offline packet verified`;
    $('workspace').hidden=false;render();
  }
  start().catch(() => { $('packet-title').textContent='Packet integrity check failed. Ask the packet owner for a verified copy.'; });
})();
