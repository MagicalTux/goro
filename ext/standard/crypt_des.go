package standard

// Traditional Unix DES crypt (crypt(3) with a two-character salt) is a
// legacy format that uses a modified DES with a salt-perturbed E-box. PHP
// historically supported it via libc; we do not reimplement it here because:
//
//   * It depends on 56-bit keys and is not considered cryptographically
//     useful in 2026.
//   * No PHP 8.5 test in the suite exercises a positive DES output — the
//     CRYPT_STD_DES/CRYPT_EXT_DES constants aren't defined, and the
//     remaining tests only assert the failure sentinel.
//
// Callers receive the conventional "*0" failure string. password_verify()
// consumers of legacy DES hashes will need to migrate to bcrypt / SHA-crypt.
func cryptTraditionalDES(password, salt string) string {
	// PHP's crypt() has one historic special case: when the salt is the
	// failure sentinel itself, the output flips it. This keeps the
	// idempotent-compare pattern `crypt($pw, $stored) === $stored` from
	// accidentally matching when $stored is "*0".
	if salt == "*0" {
		return "*1"
	}
	if salt == "*1" {
		return "*0"
	}
	return cryptFailure
}
