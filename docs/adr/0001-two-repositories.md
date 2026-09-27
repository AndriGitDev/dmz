# ADR 0001: Separate public site and product stack

Status: accepted · 27 September 2026

The marketing site lives in public `dmz-web`; the detector, applications, services, benchmark and deployment configuration live in private `dmz`. This allows transparent public messaging and legal material without exposing operational configuration or unreleased code. The website does not host customer prompts. A future pilot intake API must be hosted in Iceland and integrated only after its privacy and retention controls are reviewed.
