package validation

import (
	"fmt"
	"net"
	"strings"
)

var validDecisionTypes = map[string]bool{"ban": true, "captcha": true}
var validScopes = map[string]bool{"Ip": true, "Range": true, "Country": true}

// DecisionFields validates the type, scope, and value of a decision.
// It returns a descriptive error for invalid combinations.
func DecisionFields(decType, scope, value string) error {
	if !validDecisionTypes[decType] {
		return fmt.Errorf("invalid type %q: must be ban or captcha", decType)
	}
	if !validScopes[scope] {
		return fmt.Errorf("invalid scope %q: must be Ip, Range, or Country", scope)
	}
	if value == "" {
		return fmt.Errorf("value is required")
	}
	switch scope {
	case "Ip":
		if net.ParseIP(value) == nil {
			return fmt.Errorf("invalid IP address: %q", value)
		}
	case "Range":
		if _, _, err := net.ParseCIDR(value); err != nil {
			return fmt.Errorf("invalid CIDR range: %q", value)
		}
	case "Country":
		if len(value) != 2 || strings.ToUpper(value) != value {
			return fmt.Errorf("invalid country code: %q (must be ISO 3166-1 alpha-2, e.g. US)", value)
		}
	}
	return nil
}
