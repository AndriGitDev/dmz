package surrogates

import "github.com/AndriGitDev/dmz/packages/detect"

func supportedEntityKind(kind detect.Kind) bool {
	switch kind {
	case detect.Name, detect.Address, detect.Phone, detect.Email, detect.IBAN, detect.Card, detect.Secret:
		return true
	default:
		// Kennitala requires the dedicated Icelandic recognizer. Tenant terms
		// require the session's explicit configuration. Review signals are not
		// entities and cannot be supplied by an external model.
		return false
	}
}

func entityReview() Result {
	return Result{Allowed: false, Findings: []Finding{{detect.Kind("entity_review"), detect.Uncertain, "review", "invalid or conflicting model entities"}}}
}
