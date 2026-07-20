package tenancy

// IsValidUUID reports whether s is a canonical textual UUID
// (8-4-4-4-12 lowercase or uppercase hex groups).
func IsValidUUID(s string) bool {
	const canonicalLen = 36
	if len(s) != canonicalLen {
		return false
	}
	for i := 0; i < canonicalLen; i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}
