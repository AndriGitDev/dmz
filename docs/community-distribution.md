# DMZ Chromium extension: alpha test distribution

Owner direction: [ADR 0006](adr/0006-chromium-name-and-public-test-download.md), 27 October 2026. Publish the current trained scanner as a renamed 0.1.3 alpha.3 test package on dmz.is, without implying a store release or independent quality acceptance. The compiled distribution includes the on-device model; source, raw corpora and research remain private.

## Reproduce the named test build

1. Retrieve the unchanged private `shield-alpha-0.1.1-alpha.1` release and companion record. Check the original archive SHA-256 and every member hash before extraction. Keep those original assets intact outside the repository.
2. Run `DMZ_BASELINE_EXTENSION=/absolute/path/to/original/extension npm run build:test-release` in `apps/shield`. The script verifies the recorded baseline, all model/runtime assets and exact equality of the current core/scanner before rebuilding the renamed UI/content script. It refuses a changed detector.
3. Run type checks, existing browser workflows and trained scanner checks. Test with fabricated examples; no real prompts or confidential screenshots.
4. Commit the source, rebuild clean, then run `DMZ_RELEASE_LABEL=alpha.3 npm run record:build`. Package using `python3 scripts/package-test-release.py --output /outside/repo/dmz-chromium-extension-0.1.3-alpha.3.zip`. Clean source, manifest/profile identity, all members and ZIP integrity are required. The old Alpha 1 packager retains its original guards.
5. Retain the bilingual START-HERE, feedback and synthetic attachment kit. Preserve the model attribution and runtime/third-party notices. No raw datasets or research reports are included.
6. Stage the immutable ZIP and SHA-256 file in the website's managed persistent downloads volume. Verify the remote bytes before linking. Publish only sanitized size/version/checksum metadata in the website repository; never store the binary or host credentials in Git.
7. Verify the download end to end through www.dmz.is, install the extracted extension in Chromium and check the new name, version, local scanner, warnings and fabricated masking/restoration.

## Public installation

Use `/vidbot/` or `/en/extension/`. Download, extract and open START-HERE.html. In a Chromium browser's Extensions page, enable Developer mode and Load unpacked the `extension` subfolder. Disable older duplicate extensions and reload chat tabs. Keep the extracted folder. The AI service has its own terms/accounts/costs; no DMZ account is required.

The first scan can take several seconds. Failures warn and continue; attachments are sent unchanged. The host AI page can see the original composer draft before masking. Reset/reload/close loses previous mappings; uninstalling does not recall content already sent. This public alpha is for fabricated-data testing, not approval for confidential material.

Independent quality acceptance, supervised live-site evidence for the selected build and final store/legal/privacy claims remain separate. Preserve the research targets and all regression gates. Open-source publication is undecided; optional Kastro assistance never gates the free test download.

Alpha 3 adds active-tab temporary pause. Include `npm run test:pause` in verification alongside existing browser/scanner suites. The initial rename/test publication remains versioned as Alpha 2 and must remain available for existing links.
