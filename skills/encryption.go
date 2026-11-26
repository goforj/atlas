package skills

// payloadEncryption teaches the installed authenticated envelope and rotation boundaries without inventing secret management.
const payloadEncryption = `
# GoForj Payload Encryption

Use github.com/goforj/crypt when the application needs authenticated encrypted payloads and compatibility with the Project's APP_KEY envelope. Generated Projects currently pin v1.2.0. Inspect that version and the existing persisted format before changing encryption behavior. Keep adequate reviewed encryption code or an explicitly selected existing package when interoperability does not require a migration. Encryption is not password hashing, authorization, environment loading, or a managed secret store.

Use an instance with an injected startup key set: crypt.New(current, previous...) accepts raw 16- or 32-byte keys and copies them. crypt.NewFromEnv reads base64-prefixed APP_KEY and comma-separated APP_PREVIOUS_KEYS. Plain text keys, arbitrary 24-byte AES keys, and an unprefixed base64 string are not that API's accepted key format. Do not invent a make:crypt command or a framework resource manager; compose an application-owned cipher provider through the existing service/Wire boundary.

Cipher.Encrypt always writes with the current key and randomized IV. Cipher.Decrypt authenticates before returning plaintext and tries current followed by previous keys. The installed version writes an authenticated CBC envelope with MAC; do not describe it as AES-GCM or hand-roll encryption, padding, or authentication. Preserve the existing envelope for compatibility-sensitive stored records and downstream readers.

For example, cipher, err := crypt.New(currentKey, oldKey) creates a new-key writer that can still read records encrypted with oldKey. Retaining an old key does not reencrypt stored data. Remove previous keys only after old records and every reader's deployment lifecycle have been accounted for. Instance construction freezes keys; package-level Encrypt/Decrypt read environment configuration on each call and can lose a per-service snapshot boundary.

Encrypt successfully before writing ciphertext to repositories. Authenticate successfully before returning plaintext. Preserve context, storage, and cipher errors without returning partial plaintext or persisting submitted plaintext on failure. When the caller is already canceled, return the cancellation before invoking the cipher or repository; passing a canceled context downstream does not satisfy that no-work boundary. Use errors.Is for ErrInvalidKey, ErrInvalidPayload, and ErrAuthentication when application policy needs the library's classifications. Never log key material, plaintext, or full ciphertext envelopes as diagnostics.

Verify round trips including empty and Unicode data, ciphertext randomization, tampered IV/value/MAC rejection, wrong keys, malformed envelopes, old-key reads after rotation, and new writes decryptable by the current key alone. Use fake keys and a compatibility fixture, and prove operational failures through injected repository/cipher seams. Key validation belongs before readiness; there is no external account action or real credential rotation in a local test.
`
