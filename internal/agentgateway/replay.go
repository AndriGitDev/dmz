package agentgateway

// TransformReplyHistory re-masks only exact values previously restored locally
// in ordinary assistant and tool-result text. It never detects new entities in
// those fields and never touches tool calls, signed thinking, encrypted state or
// attachments. User text is additionally protected in prepare's masking callback.
func TransformReplyHistory(protocol Protocol, body []byte, remask MaskFunc) ([]byte, TransformReport) {
	t := &transformer{body: body, mask: remask}
	if protocol != Responses && protocol != Messages {
		t.warn(WarnUnsupportedProtocol)
		return body, t.report
	}
	root, perr := parseJSON(body)
	if perr != parseOK {
		codes := map[parseError]string{parseTrailing: WarnTrailingData, parseLoneSurrogate: WarnLoneSurrogate, parseDuplicateKey: WarnDuplicateKey, parseTooDeep: WarnNestingTooDeep, parseTooComplex: WarnJSONTooComplex}
		code := codes[perr]
		if code == "" {
			code = WarnInvalidJSON
		}
		t.warn(code)
		return body, t.report
	}
	if root.kind != kindObject {
		t.warn(WarnUnsupportedShape)
		return body, t.report
	}
	key := "input"
	if protocol == Messages {
		key = "messages"
	}
	items := root.get(key)
	if items == nil || items.kind != kindArray {
		return body, t.report
	}
	for _, item := range items.elems {
		if protocol == Responses {
			if typ, _ := t.str(item.get("type")); isToolOutputType(typ) {
				t.replayToolContent(item.get("output"), protocol)
				if typ == "shell_call_output" {
					if output := item.get("output"); output != nil && output.kind == kindArray {
						for _, entry := range output.elems {
							for _, field := range []string{"stdout", "stderr"} {
								if value := entry.get(field); value != nil && value.kind == kindString {
									t.pending = append(t.pending, value)
								}
							}
						}
					}
				}
				continue
			}
		}
		if protocol == Messages {
			if role, _ := t.str(item.get("role")); role == "user" {
				if content := item.get("content"); content != nil && content.kind == kindArray {
					for _, block := range content.elems {
						if typ, _ := t.str(block.get("type")); typ == "tool_result" {
							t.replayToolContent(block.get("content"), protocol)
						}
					}
				}
			}
		}
		if role, _ := t.str(item.get("role")); role != "assistant" {
			continue
		}
		if protocol == Responses {
			if typ := item.get("type"); typ != nil {
				if name, _ := t.str(typ); name != "message" {
					continue
				}
			}
		}
		content := item.get("content")
		if content == nil {
			continue
		}
		if content.kind == kindString {
			t.pending = append(t.pending, content)
			continue
		}
		if content.kind != kindArray {
			continue
		}
		for _, part := range content.elems {
			typ, _ := t.str(part.get("type"))
			eligible := protocol == Responses && (typ == "input_text" || typ == "output_text") || protocol == Messages && typ == "text"
			if !eligible {
				continue
			}
			if text := part.get("text"); text != nil && text.kind == kindString {
				t.pending = append(t.pending, text)
			}
		}
	}
	t.run()
	if len(t.edits) == 0 {
		return body, t.report
	}
	return t.apply(), t.report
}

func (t *transformer) replayToolContent(content *jsonNode, protocol Protocol) {
	if content == nil {
		return
	}
	if content.kind == kindString {
		t.pending = append(t.pending, content)
		return
	}
	if content.kind != kindArray {
		return
	}
	for _, part := range content.elems {
		typ, _ := t.str(part.get("type"))
		if protocol == Responses && (typ == "input_text" || typ == "output_text" || typ == "text") || protocol == Messages && typ == "text" {
			if text := part.get("text"); text != nil && text.kind == kindString {
				t.pending = append(t.pending, text)
			}
		}
	}
}
