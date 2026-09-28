# ADR 0003: Kennitala detection after check-digit changes

Date: 28 September 2026
Status: accepted for the prototype

The business plan's detector table requires a mod-11 check digit. [Ísland.is](https://island.is/en/news/release-17th-of-february-2026) announced that systems must support kennitölur without calculated check digits from 18 February 2026. The editorial update to [Vísindavefurinn](https://www.visindavefur.is/svar.php?id=183) says the calculation was abolished for newly registered identifiers.

The detector now validates date, legal-entity day offset and century digit. A matching legacy check digit is `high` confidence. A plausible date-shaped identifier with a nonmatching check digit is `uncertain` and is still masked by the prototype. This avoids silently sending a plausible newer kennitala. The uncertain class can contain false positives and must be measured separately as the corpus grows. Test kennitölur still use century digit `8` so committed examples cannot belong to living people.
