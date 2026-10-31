# ADR 0003: Kennitala detection after check-digit changes

Date: 28 September 2026
Status: accepted for the prototype


The detector now validates date, legal-entity day offset and century digit. A matching legacy check digit is `high` confidence. A plausible date-shaped identifier with a nonmatching check digit is `uncertain` and is still masked by the prototype. This avoids silently sending a plausible newer kennitala. The uncertain class can contain false positives and must be measured separately as the corpus grows. Test kennitölur still use century digit `8` so committed examples cannot belong to living people.
