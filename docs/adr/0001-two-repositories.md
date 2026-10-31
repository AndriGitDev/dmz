# ADR 0001: Separate website and extension repositories

Status: updated by owner direction, 31 October 2026.

`dmz-web` contains the bilingual website; `dmz` contains the Chromium extension, detector and supporting tools. Original code in both uses GPL-3.0-only. Public visibility is a separate publication step after [history and asset review](../open-source-readiness.md).

The website does not collect prompts. Keep real user data, credentials and licensed source datasets outside both repositories. Public detector content remains limited to reviewed summaries and honest limitations.
