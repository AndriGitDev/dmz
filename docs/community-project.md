# DMZ: a free community privacy tool

Direction set: 27 October 2026. This document is the active concept and scope reset, following the owner's request to make DMZ a free community project like Dmark and Kastro Lookup. It supersedes commercial priorities, paid-pilot gates and expansion commitments in the September business plan. It does not certify a release or change runtime behaviour.

## Purpose

Help people reduce accidental sharing of direct identifiers and secrets when using AI, with a practical tool that works locally and explains its limits. Start with Icelandic text and the existing Shield extension; make the experience understandable in Icelandic and English.

Working positioning: **DMZ — a free tool from Kastro for masking sensitive details before sharing text with AI.**

The useful outcome is that someone can install Shield, understand what it changes, and use its supported text workflow without an account, payment or sales conversation. Detection is best effort. DMZ does not guarantee anonymity, complete detection or safe use of confidential material.

## What free means

- One free community capability set: no subscriptions, seat minimums, paid security tier or upgrade prompts.
- No DMZ account requirement for local Shield use. The user's chosen AI service has its own account requirements and costs; DMZ does not supply AI inference or bypass provider subscriptions.
- No prompt collection, sale of user data, advertising-funded detection or mandatory lead form. Keep prompt text, original values and restoration mappings on the device for Shield processing. The AI service still receives submitted text, including anything missed or sent unchanged.
- Optional Kastro implementation or training help may be quoted separately. Getting useful protection and guidance must never require buying that help.
- Free access does not imply open-source code, public model weights, unrestricted dataset redistribution, guaranteed support or indefinite funding. Those require separate decisions and evidence.

This borrows Dmark's free utility and optional assistance model and Lookup's focused, accessible utility model. It does not import their account, storage or server architecture into Shield.

## First product: Shield

Shield is the product, rather than one tier in a three-product sales funnel. Retain the existing device-local detector, visible masks, restoration, supported ChatGPT/Claude text workflows and honest warnings. Make installation, supported browsers/sites, model download size and fallback behaviour clear before use.

Current status: a prototype and colleague alpha exist, including a separately packaged trained scanner. That is not a generally available store release. Independent detector acceptance remains incomplete; the existing research targets and no-regression requirements remain in force. Verify actual packages and store status before changing any availability claim.

Under the 9 October owner policy, Shield attempts masking and continues submission with a warning when scanning or composer handling fails. Attachments are sent unchanged with a warning. Unsupported paths and site changes can bypass interception. Present those limits near installation and use; never describe Shield as an enforcement boundary. Reducing scope or making the project free does not establish detector quality.

## Community participation

Start with low-overhead participation: installation feedback, browser compatibility reports, translation corrections and fabricated examples of misses or excessive masking. Publish only a verified reporting/contact route. Contributors should never submit real prompts, personal identifiers, secrets or customer logs. Use synthetic century-digit-8 kennitölur in examples.

A maintainer reviews reports, reproduces them privately where appropriate and publishes concise release notes. Keep a private security-reporting route separate from ordinary feedback. Do not invite public uploads until redaction guidance and moderation are ready. No forum, account system or public issue tracker is required for the first release.

Keep reviewed detector summaries public, with method and limitations. Existing private research, training provenance, weights, recipes and complete challenge material stay private under the current disclosure policy. Public contribution scope and licensing must be decided before inviting code or model contributions.

## Scope boundaries

| Prior direction | Community direction |
| --- | --- |
| Shield, Portal and Gateway sold as a product suite | Shield first; Portal/Gateway deferred, with no delivery commitment |
| Paid pilots, per-seat plans and minimum seats | Free local use and voluntary feedback |
| Procurement/compliance-led sales funnel | Installation, practical explanation, measured limitations and help |
| MRR, customer counts and expansion gates | Successful use, understandable warnings, repeat value and maintainability |
| Hosted inference, routing, tenant vault and audit programme | Outside the initial community release |
| Certification and ZDR roadmap as selling points | No new certification, compliance or provider-retention claims |

Keep existing code and historical plans for reference. Deferring hosted features does not authorize deleting infrastructure, changing any real user dependency or publishing the private repository. If hosted work is reconsidered, ADR 0004 and its unresolved evidence requirements still apply.

## Website transformation brief

Reframe dmz.is around one primary action: **Try Shield**, linked only to a verified artifact or published store listing. Until one is ready, show an accurate preview status and installation guidance. A local paste-and-mask demo is a possible later improvement, not a launch promise.

The first public copy pass should:

1. Replace the suite/pricing/pilot funnel with free Shield, how it works, current limits, installation and community feedback.
2. Replace pricing content with a free-access explanation and redirect or retain useful old URLs in both languages. Remove indicative prices and seat minimums from navigation, metadata and structured content.
3. Remove Portal/Gateway from active product navigation; explain their deferred status wherever retained pages remain reachable. Do not imply a hosted service exists.
4. Update about, contact and privacy text for the actual operator and distribution path. Preserve security contact and applicable privacy/legal information; remove procurement documents from primary navigation only after reviewing their role.
5. Preserve Icelandic/English parity, accessibility, synthetic-only demonstrations, current detector-disclosure limits and the no-tracker baseline.

The cross-repository transition implements this brief in the website, extension identity/evaluation guidance and Kastro references. Website publication and extension distribution must be verified separately; this scope decision alone certifies neither.

## First release path

1. **Scope and messaging:** adopt this concept, inventory existing downloads and all public price/pilot/product claims, then update both website locales.
2. **Distribution and usability:** verify the chosen extension build, rights to redistribute every bundled asset, installation/update/uninstall instructions, supported paths, model size and fallback warnings. Store submission is a separate release action.
3. **Quality and feedback:** run existing regression and browser workflow checks; obtain independent quality evidence before broader safety claims. Test that people understand masking, restoration, misses, unchanged attachments and failed-scan warnings using fabricated text.
4. **Maintainable community release:** publish reviewed limitations and release notes, verify reporting routes, set a realistic support boundary and review maintenance/distribution costs. Expand only when usage and available maintenance capacity justify it.

Assess value through voluntary feedback and synthetic usability sessions first: can users complete the workflow, understand failures and return because it helps? No mandatory telemetry or collection of their prompts. Set numerical adoption goals after a baseline exists; the old three-paid-pilot and MRR gates no longer decide whether this project proceeds.

## Decisions still needed

- Confirm the public operator, support/security contacts and sustainable maintenance budget.
- Choose the first distributable build and release channel after checking asset rights and independent quality limitations.
- Decide whether any code will be open source, its license, contribution boundaries and third-party redistribution rights. Until then the product repository and research remain private.
- Decide whether donations or sponsorship are useful later; neither is required for free use or part of this first scope.
